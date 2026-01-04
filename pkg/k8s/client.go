package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"coredns-multi-configuration/pkg/models"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Manager manages Kubernetes client connections for multiple clusters
type Manager struct {
	mu      sync.RWMutex
	clients map[string]*kubernetes.Clientset
}

// NewManager creates a new K8s client manager
func NewManager() *Manager {
	return &Manager{
		clients: make(map[string]*kubernetes.Clientset),
	}
}

// GetClient returns a K8s client for the specified cluster
func (m *Manager) GetClient(cluster *models.Cluster) (*kubernetes.Clientset, error) {
	m.mu.RLock()
	client, exists := m.clients[cluster.ID]
	m.mu.RUnlock()

	if exists {
		return client, nil
	}

	// Create new client
	client, err := m.createClient(cluster)
	if err != nil {
		return nil, err
	}

	m.mu.Lock()
	m.clients[cluster.ID] = client
	m.mu.Unlock()

	return client, nil
}

// RemoveClient removes a cached client for the specified cluster
func (m *Manager) RemoveClient(clusterID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.clients, clusterID)
}

// createClient creates a new Kubernetes client from kubeconfig
func (m *Manager) createClient(cluster *models.Cluster) (*kubernetes.Clientset, error) {
	// Decode base64 kubeconfig
	kubeconfigData, err := base64.StdEncoding.DecodeString(cluster.Kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to decode kubeconfig: %w", err)
	}

	// Build config from kubeconfig
	config, err := clientcmd.RESTConfigFromKubeConfig(kubeconfigData)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return clientset, nil
}

// TestConnection tests the connection to a cluster
func (m *Manager) TestConnection(ctx context.Context, cluster *models.Cluster) error {
	client, err := m.GetClient(cluster)
	if err != nil {
		return err
	}

	// Try to get server version as a connectivity test
	_, err = client.Discovery().ServerVersion()
	if err != nil {
		// Remove cached client on failure
		m.RemoveClient(cluster.ID)
		return fmt.Errorf("failed to connect to cluster: %w", err)
	}

	return nil
}
