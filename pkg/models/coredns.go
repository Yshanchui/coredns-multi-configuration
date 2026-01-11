package models

import (
	"fmt"
	"strings"
)

// ForwardRule represents a CoreDNS forward rule for cross-cluster DNS resolution
type ForwardRule struct {
	Namespace   string `json:"namespace"`              // e.g., "prod", "tidb-cluster"
	ServiceName string `json:"service_name,omitempty"` // e.g., "mysql" (optional, for service-level rules)
	TargetIP    string `json:"target_ip"`              // target CoreDNS IP, e.g., "10.96.0.10"
	IsFullFQDN  bool   `json:"is_full_fqdn,omitempty"` // true if input was *.svc.cluster.local format
}

// GetFullName returns the full name (service.namespace or just namespace)
func (r *ForwardRule) GetFullName() string {
	if r.ServiceName != "" {
		return r.ServiceName + "." + r.Namespace
	}
	return r.Namespace
}

// GetFQDN returns the full qualified domain name
func (r *ForwardRule) GetFQDN() string {
	return r.GetFullName() + ".svc.cluster.local"
}

// GetDomainBlock returns the domain block format used in corefile
// Short format: service.namespace:53 or namespace:53
// FQDN format: *.svc.cluster.local:53
func (r *ForwardRule) GetDomainBlock() string {
	if r.IsFullFQDN {
		return r.GetFQDN() + ":53"
	}
	return r.GetFullName() + ":53"
}

// ToCorefile generates the Corefile block for this forward rule
// Formats:
// 1. service.namespace (mysql.mysql) -> mysql.mysql:53 { rewrite exact ... }
// 2. namespace only (mysql) -> mysql:53 { rewrite regex ... }
// 3. *.svc.cluster.local -> full FQDN:53 { forward only }
func (r *ForwardRule) ToCorefile() string {
	if r.IsFullFQDN {
		// Direct FQDN input - only forward, use full FQDN for domain
		fqdn := r.GetFullName() + ".svc.cluster.local"
		return fmt.Sprintf(`%s:53 {
    forward . %s
}`, fqdn, r.TargetIP)
	}

	fullName := r.GetFullName()
	fullFQDN := fullName + ".svc.cluster.local."

	if r.ServiceName != "" {
		// Service.namespace format (e.g., mysql.mysql)
		// Domain: mysql.mysql:53, rewrite exact (no regex patterns)
		return fmt.Sprintf(`%s:53 {
    rewrite name exact %s %s answer auto
    forward . %s
}`, fullName, fullName, fullFQDN, r.TargetIP)
	}

	// Namespace only format (e.g., mysql)
	// Domain: mysql:53, rewrite regex for all services
	return fmt.Sprintf(`%s:53 {
    rewrite name regex (.*)\.%s %s.svc.cluster.local. answer auto
    forward . %s
}`, r.Namespace, r.Namespace, r.Namespace, r.TargetIP)
}

// ParseNameInput parses user input like "namespace", "service.namespace",
// "namespace.svc.cluster.local", or "service.namespace.svc.cluster.local"
// Returns (serviceName, namespace, isFullFQDN)
func ParseNameInput(input string) (serviceName, namespace string, isFullFQDN bool) {
	input = strings.TrimSpace(input)

	// Check if input ends with .svc.cluster.local
	if strings.HasSuffix(input, ".svc.cluster.local") {
		isFullFQDN = true
		input = strings.TrimSuffix(input, ".svc.cluster.local")
	}

	parts := strings.SplitN(input, ".", 2)
	if len(parts) == 2 {
		return parts[0], parts[1], isFullFQDN
	}
	return "", parts[0], isFullFQDN
}

// CoreDNSConfig represents the CoreDNS configuration for a cluster
type CoreDNSConfig struct {
	ClusterID    string        `json:"cluster_id"`
	Corefile     string        `json:"corefile"`
	ForwardRules []ForwardRule `json:"forward_rules"`
}
