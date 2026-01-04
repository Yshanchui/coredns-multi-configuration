package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"coredns-multi-configuration/pkg/models"

	"github.com/google/uuid"
)

// Store provides JSON file-based storage for application data
type Store struct {
	dataDir  string
	mu       sync.RWMutex
	clusters []models.Cluster
}

// New creates a new Store instance
func New(dataDir string) (*Store, error) {
	// Ensure data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, err
	}

	s := &Store{
		dataDir:  dataDir,
		clusters: make([]models.Cluster, 0),
	}

	// Load existing data
	if err := s.load(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Store) clustersFile() string {
	return filepath.Join(s.dataDir, "clusters.json")
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Load clusters
	data, err := os.ReadFile(s.clustersFile())
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No data yet
		}
		return err
	}

	return json.Unmarshal(data, &s.clusters)
}

func (s *Store) save() error {
	data, err := json.MarshalIndent(s.clusters, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.clustersFile(), data, 0644)
}

// GetClusters returns all clusters
func (s *Store) GetClusters() []models.Cluster {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]models.Cluster, len(s.clusters))
	copy(result, s.clusters)
	return result
}

// GetCluster returns a cluster by ID
func (s *Store) GetCluster(id string) (*models.Cluster, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, c := range s.clusters {
		if c.ID == id {
			return &c, true
		}
	}
	return nil, false
}

// AddCluster adds a new cluster
func (s *Store) AddCluster(cluster models.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if cluster.ID == "" {
		cluster.ID = uuid.New().String()
	}
	s.clusters = append(s.clusters, cluster)
	return s.save()
}

// DeleteCluster deletes a cluster by ID
func (s *Store) DeleteCluster(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, c := range s.clusters {
		if c.ID == id {
			s.clusters = append(s.clusters[:i], s.clusters[i+1:]...)
			return s.save()
		}
	}
	return nil
}

// UpdateCluster updates an existing cluster
func (s *Store) UpdateCluster(cluster models.Cluster) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, c := range s.clusters {
		if c.ID == cluster.ID {
			s.clusters[i] = cluster
			return s.save()
		}
	}
	return nil
}
