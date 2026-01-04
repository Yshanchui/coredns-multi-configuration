package models

import "fmt"

// ForwardRule represents a CoreDNS forward rule for cross-cluster DNS resolution
type ForwardRule struct {
	Namespace string `json:"namespace"` // e.g., "prod", "staging"
	TargetIP  string `json:"target_ip"` // target CoreDNS IP, e.g., "10.96.0.10"
}

// ToCorefile generates the Corefile block for this forward rule
func (r *ForwardRule) ToCorefile() string {
	return fmt.Sprintf(`%s.svc.cluster.local:53 {
    forward . %s
}`, r.Namespace, r.TargetIP)
}

// CoreDNSConfig represents the CoreDNS configuration for a cluster
type CoreDNSConfig struct {
	ClusterID    string        `json:"cluster_id"`
	Corefile     string        `json:"corefile"`
	ForwardRules []ForwardRule `json:"forward_rules"`
}
