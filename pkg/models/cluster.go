package models

import "time"

// Cluster represents a Kubernetes cluster configuration
type Cluster struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Kubeconfig string    `json:"kubeconfig"` // base64 encoded
	CreatedAt  time.Time `json:"created_at"`
}

// ClusterStatus represents the connection status of a cluster
type ClusterStatus struct {
	Connected bool   `json:"connected"`
	Error     string `json:"error,omitempty"`
}
