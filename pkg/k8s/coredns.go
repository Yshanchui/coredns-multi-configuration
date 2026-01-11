package k8s

import (
	"context"
	"fmt"
	"strings"

	"coredns-multi-configuration/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	CoreDNSNamespace     = "kube-system"
	CoreDNSConfigMapName = "coredns"
	KubeDNSServiceName   = "kube-dns"
	CorefileName         = "Corefile"
)

// CoreDNSHandler handles CoreDNS configuration operations
type CoreDNSHandler struct {
	manager *Manager
}

// NewCoreDNSHandler creates a new CoreDNS handler
func NewCoreDNSHandler(manager *Manager) *CoreDNSHandler {
	return &CoreDNSHandler{manager: manager}
}

// CoreDNSInfo contains CoreDNS configuration and service information
type CoreDNSInfo struct {
	ConfigMap    *corev1.ConfigMap    `json:"configmap"`
	Service      *corev1.Service      `json:"service"`
	Corefile     string               `json:"corefile"`
	ServiceIP    string               `json:"service_ip"`
	ForwardRules []models.ForwardRule `json:"forward_rules"`
}

// GetCoreDNSInfo retrieves CoreDNS configuration and service info from a cluster
func (h *CoreDNSHandler) GetCoreDNSInfo(ctx context.Context, cluster *models.Cluster) (*CoreDNSInfo, error) {
	client, err := h.manager.GetClient(cluster)
	if err != nil {
		return nil, err
	}

	// Get CoreDNS ConfigMap
	configMap, err := client.CoreV1().ConfigMaps(CoreDNSNamespace).Get(ctx, CoreDNSConfigMapName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get coredns configmap: %w", err)
	}

	// Get kube-dns Service
	service, err := client.CoreV1().Services(CoreDNSNamespace).Get(ctx, KubeDNSServiceName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get kube-dns service: %w", err)
	}

	info := &CoreDNSInfo{
		ConfigMap: configMap,
		Service:   service,
		Corefile:  configMap.Data[CorefileName],
		ServiceIP: service.Spec.ClusterIP,
	}

	// Parse existing forward rules from Corefile
	info.ForwardRules = parseForwardRules(info.Corefile)

	return info, nil
}

// UpdateCorefile updates the CoreDNS Corefile configuration
func (h *CoreDNSHandler) UpdateCorefile(ctx context.Context, cluster *models.Cluster, corefile string) error {
	client, err := h.manager.GetClient(cluster)
	if err != nil {
		return err
	}

	// Get current ConfigMap
	configMap, err := client.CoreV1().ConfigMaps(CoreDNSNamespace).Get(ctx, CoreDNSConfigMapName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get coredns configmap: %w", err)
	}

	// Update Corefile
	configMap.Data[CorefileName] = corefile

	// Apply update
	_, err = client.CoreV1().ConfigMaps(CoreDNSNamespace).Update(ctx, configMap, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to update coredns configmap: %w", err)
	}

	return nil
}

// AddForwardRule adds a forward rule to the CoreDNS configuration
func (h *CoreDNSHandler) AddForwardRule(ctx context.Context, cluster *models.Cluster, rule models.ForwardRule) error {
	info, err := h.GetCoreDNSInfo(ctx, cluster)
	if err != nil {
		return err
	}

	// Check if rule already exists (compare full name: service.namespace or just namespace)
	for _, r := range info.ForwardRules {
		if r.GetFullName() == rule.GetFullName() {
			return fmt.Errorf("forward rule for %s already exists", rule.GetFullName())
		}
	}

	// Append new rule to Corefile
	newCorefile := info.Corefile + "\n" + rule.ToCorefile() + "\n"

	return h.UpdateCorefile(ctx, cluster, newCorefile)
}

// DeleteForwardRule removes a forward rule from the CoreDNS configuration
// The name parameter can be "namespace" or "service.namespace"
// isFullFQDN indicates whether the rule uses FQDN format (*.svc.cluster.local:53)
func (h *CoreDNSHandler) DeleteForwardRule(ctx context.Context, cluster *models.Cluster, name string, isFullFQDN bool) error {
	info, err := h.GetCoreDNSInfo(ctx, cluster)
	if err != nil {
		return err
	}

	// Parse input and build the rule block pattern
	serviceName, namespace, _ := models.ParseNameInput(name)
	var fullName string
	if serviceName != "" {
		fullName = serviceName + "." + namespace
	} else {
		fullName = namespace
	}

	// Build domain block pattern based on format type
	var ruleBlock string
	if isFullFQDN {
		ruleBlock = fmt.Sprintf("%s.svc.cluster.local:53", fullName)
	} else {
		ruleBlock = fmt.Sprintf("%s:53", fullName)
	}

	lines := strings.Split(info.Corefile, "\n")
	var newLines []string
	skipBlock := false
	braceCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, ruleBlock) {
			skipBlock = true
			braceCount = 0
		}

		if skipBlock {
			braceCount += strings.Count(line, "{")
			braceCount -= strings.Count(line, "}")
			if braceCount <= 0 && strings.Contains(line, "}") {
				skipBlock = false
				continue
			}
			continue
		}

		newLines = append(newLines, line)
	}

	newCorefile := strings.Join(newLines, "\n")
	return h.UpdateCorefile(ctx, cluster, newCorefile)
}

// parseForwardRules extracts forward rules from a Corefile
// Supports 4 domain formats:
// 1. namespace:53 (short format, namespace only)
// 2. service.namespace:53 (short format, service.namespace)
// 3. namespace.svc.cluster.local:53 (FQDN format)
// 4. service.namespace.svc.cluster.local:53 (FQDN format)
func parseForwardRules(corefile string) []models.ForwardRule {
	var rules []models.ForwardRule
	lines := strings.Split(corefile, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for patterns ending with :53 { or :53{
		if !strings.Contains(trimmed, ":53") {
			continue
		}
		if !strings.HasSuffix(trimmed, "{") && !strings.HasSuffix(trimmed, "{ ") {
			continue
		}

		// Extract the domain part before :53
		domainPart := strings.TrimSuffix(trimmed, "{")
		domainPart = strings.TrimSuffix(domainPart, " ")
		domainPart = strings.TrimSuffix(domainPart, ":53")
		domainPart = strings.TrimSpace(domainPart)

		// Skip main zones
		if domainPart == "" || domainPart == "." || domainPart == "cluster.local" {
			continue
		}

		var serviceName, namespace string
		var isFullFQDN bool

		if strings.HasSuffix(domainPart, ".svc.cluster.local") {
			// FQDN format
			isFullFQDN = true
			name := strings.TrimSuffix(domainPart, ".svc.cluster.local")
			serviceName, namespace, _ = models.ParseNameInput(name)
		} else {
			// Short format (namespace or service.namespace)
			parts := strings.SplitN(domainPart, ".", 2)
			if len(parts) == 2 {
				serviceName = parts[0]
				namespace = parts[1]
			} else {
				namespace = parts[0]
			}
		}

		// Skip if namespace is empty
		if namespace == "" {
			continue
		}

		// Look for forward line in the next few lines
		for j := i + 1; j < len(lines) && j < i+10; j++ {
			forwardLine := strings.TrimSpace(lines[j])
			if strings.HasPrefix(forwardLine, "forward .") {
				// Extract target IP
				forwardParts := strings.Fields(forwardLine)
				if len(forwardParts) >= 3 {
					rules = append(rules, models.ForwardRule{
						Namespace:   namespace,
						ServiceName: serviceName,
						TargetIP:    forwardParts[2],
						IsFullFQDN:  isFullFQDN,
					})
				}
				break
			}
			if strings.Contains(forwardLine, "}") {
				break
			}
		}
	}

	return rules
}

// GetDeployment retrieves the CoreDNS deployment info
func (h *CoreDNSHandler) GetDeployment(ctx context.Context, client *kubernetes.Clientset) (*corev1.PodList, error) {
	pods, err := client.CoreV1().Pods(CoreDNSNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-dns",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list coredns pods: %w", err)
	}
	return pods, nil
}
