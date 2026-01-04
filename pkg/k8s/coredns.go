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

	// Check if rule already exists
	for _, r := range info.ForwardRules {
		if r.Namespace == rule.Namespace {
			return fmt.Errorf("forward rule for namespace %s already exists", rule.Namespace)
		}
	}

	// Append new rule to Corefile
	newCorefile := info.Corefile + "\n" + rule.ToCorefile() + "\n"

	return h.UpdateCorefile(ctx, cluster, newCorefile)
}

// DeleteForwardRule removes a forward rule from the CoreDNS configuration
func (h *CoreDNSHandler) DeleteForwardRule(ctx context.Context, cluster *models.Cluster, namespace string) error {
	info, err := h.GetCoreDNSInfo(ctx, cluster)
	if err != nil {
		return err
	}

	// Find and remove the rule
	ruleBlock := fmt.Sprintf("%s.svc.cluster.local:53", namespace)
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
func parseForwardRules(corefile string) []models.ForwardRule {
	var rules []models.ForwardRule
	lines := strings.Split(corefile, "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Look for patterns like "namespace.svc.cluster.local:53 {"
		if strings.HasSuffix(trimmed, ".svc.cluster.local:53 {") || strings.HasSuffix(trimmed, ".svc.cluster.local:53{") {
			// Extract namespace
			parts := strings.Split(trimmed, ".")
			if len(parts) > 0 {
				namespace := parts[0]

				// Look for forward line in the next few lines
				for j := i + 1; j < len(lines) && j < i+5; j++ {
					forwardLine := strings.TrimSpace(lines[j])
					if strings.HasPrefix(forwardLine, "forward .") || strings.HasPrefix(forwardLine, "forward .") {
						// Extract target IP
						forwardParts := strings.Fields(forwardLine)
						if len(forwardParts) >= 3 {
							rules = append(rules, models.ForwardRule{
								Namespace: namespace,
								TargetIP:  forwardParts[2],
							})
						}
						break
					}
					if strings.Contains(forwardLine, "}") {
						break
					}
				}
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
