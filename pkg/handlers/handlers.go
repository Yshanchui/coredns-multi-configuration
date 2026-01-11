package handlers

import (
	"context"
	"encoding/base64"
	"net/http"
	"time"

	"coredns-multi-configuration/pkg/auth"
	"coredns-multi-configuration/pkg/config"
	"coredns-multi-configuration/pkg/k8s"
	"coredns-multi-configuration/pkg/models"
	"coredns-multi-configuration/pkg/store"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Handlers contains all HTTP handlers
type Handlers struct {
	config         *config.Config
	store          *store.Store
	auth           *auth.Auth
	k8sManager     *k8s.Manager
	coreDNSHandler *k8s.CoreDNSHandler
}

// New creates a new Handlers instance
func New(cfg *config.Config, store *store.Store, auth *auth.Auth, k8sManager *k8s.Manager) *Handlers {
	return &Handlers{
		config:         cfg,
		store:          store,
		auth:           auth,
		k8sManager:     k8sManager,
		coreDNSHandler: k8s.NewCoreDNSHandler(k8sManager),
	}
}

// ============== Auth Handlers ==============

// Login handles user login
func (h *Handlers) Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if err := h.auth.ValidateCredentials(req.Username, req.Password); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token, err := h.auth.GenerateToken(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate token"})
		return
	}

	// Set token as cookie
	c.SetCookie("token", token, 86400, "/", "", false, true)

	c.JSON(http.StatusOK, gin.H{
		"token":   token,
		"message": "login successful",
	})
}

// Logout handles user logout
func (h *Handlers) Logout(c *gin.Context) {
	c.SetCookie("token", "", -1, "/", "", false, true)
	c.Redirect(http.StatusTemporaryRedirect, "/login")
}

// ============== Cluster Handlers ==============

// ListClusters returns all clusters
func (h *Handlers) ListClusters(c *gin.Context) {
	clusters := h.store.GetClusters()

	// Add connection status for each cluster
	type ClusterWithStatus struct {
		models.Cluster
		Connected bool   `json:"connected"`
		Error     string `json:"error,omitempty"`
	}

	result := make([]ClusterWithStatus, 0, len(clusters))
	for _, cluster := range clusters {
		cws := ClusterWithStatus{Cluster: cluster}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		if err := h.k8sManager.TestConnection(ctx, &cluster); err != nil {
			cws.Error = err.Error()
		} else {
			cws.Connected = true
		}
		cancel()

		// Don't expose kubeconfig
		cws.Kubeconfig = ""
		result = append(result, cws)
	}

	c.JSON(http.StatusOK, result)
}

// AddClusterRequest represents add cluster request
type AddClusterRequest struct {
	Name       string `json:"name" binding:"required"`
	Kubeconfig string `json:"kubeconfig" binding:"required"` // Can be base64 or plain text
}

// AddCluster adds a new cluster
func (h *Handlers) AddCluster(c *gin.Context) {
	var req AddClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request: " + err.Error()})
		return
	}

	// Check if kubeconfig is already base64 encoded
	kubeconfig := req.Kubeconfig
	if _, err := base64.StdEncoding.DecodeString(kubeconfig); err != nil {
		// Not base64, encode it
		kubeconfig = base64.StdEncoding.EncodeToString([]byte(kubeconfig))
	}

	cluster := models.Cluster{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Kubeconfig: kubeconfig,
		CreatedAt:  time.Now(),
	}

	// Test connection before saving
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.k8sManager.TestConnection(ctx, &cluster); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to connect to cluster: " + err.Error()})
		return
	}

	if err := h.store.AddCluster(cluster); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save cluster"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "cluster added successfully",
		"id":      cluster.ID,
	})
}

// DeleteCluster deletes a cluster
func (h *Handlers) DeleteCluster(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "cluster id required"})
		return
	}

	h.k8sManager.RemoveClient(id)

	if err := h.store.DeleteCluster(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete cluster"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "cluster deleted successfully"})
}

// ============== CoreDNS Handlers ==============

// GetCoreDNSConfig returns CoreDNS configuration for a cluster
func (h *Handlers) GetCoreDNSConfig(c *gin.Context) {
	id := c.Param("id")
	cluster, found := h.store.GetCluster(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	info, err := h.coreDNSHandler.GetCoreDNSInfo(ctx, cluster)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, info)
}

// UpdateCorefileRequest represents update corefile request
type UpdateCorefileRequest struct {
	Corefile string `json:"corefile" binding:"required"`
}

// UpdateCorefile updates the CoreDNS Corefile
func (h *Handlers) UpdateCorefile(c *gin.Context) {
	id := c.Param("id")
	cluster, found := h.store.GetCluster(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}

	var req UpdateCorefileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.coreDNSHandler.UpdateCorefile(ctx, cluster, req.Corefile); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "corefile updated successfully"})
}

// AddForwardRuleRequest represents add forward rule request
type AddForwardRuleRequest struct {
	Namespace string `json:"namespace" binding:"required"`
	TargetIP  string `json:"target_ip" binding:"required"`
}

// AddForwardRule adds a forward rule to CoreDNS
func (h *Handlers) AddForwardRule(c *gin.Context) {
	id := c.Param("id")
	cluster, found := h.store.GetCluster(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}

	var req AddForwardRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// Parse namespace input (can be "namespace", "service.namespace", or "*.svc.cluster.local")
	serviceName, namespace, isFullFQDN := models.ParseNameInput(req.Namespace)

	rule := models.ForwardRule{
		Namespace:   namespace,
		ServiceName: serviceName,
		TargetIP:    req.TargetIP,
		IsFullFQDN:  isFullFQDN,
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.coreDNSHandler.AddForwardRule(ctx, cluster, rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "forward rule added successfully"})
}

// DeleteForwardRule removes a forward rule from CoreDNS
func (h *Handlers) DeleteForwardRule(c *gin.Context) {
	id := c.Param("id")
	name := c.Param("namespace")
	isFullFQDN := c.Query("fqdn") == "true"

	cluster, found := h.store.GetCluster(id)
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "cluster not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	if err := h.coreDNSHandler.DeleteForwardRule(ctx, cluster, name, isFullFQDN); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "forward rule deleted successfully"})
}
