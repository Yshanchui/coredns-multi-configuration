package main

import (
	"fmt"
	"log"
	"net/http"

	"coredns-multi-configuration/pkg/auth"
	"coredns-multi-configuration/pkg/config"
	"coredns-multi-configuration/pkg/handlers"
	"coredns-multi-configuration/pkg/k8s"
	"coredns-multi-configuration/pkg/store"
	"coredns-multi-configuration/templates"

	"github.com/a-h/templ"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load configuration
	cfg, err := config.Load("config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Initialize store
	dataStore, err := store.New(cfg.DataDir)
	if err != nil {
		log.Fatalf("Failed to initialize store: %v", err)
	}

	// Initialize auth
	authService := auth.New(&cfg.Auth)

	// Initialize K8s manager
	k8sManager := k8s.NewManager()

	// Initialize handlers
	h := handlers.New(cfg, dataStore, authService, k8sManager)

	// Setup Gin router
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Auth middleware
	r.Use(authService.Middleware())

	// Page routes
	r.GET("/login", func(c *gin.Context) {
		render(c, http.StatusOK, templates.LoginPage())
	})

	r.GET("/", func(c *gin.Context) {
		render(c, http.StatusOK, templates.DashboardPage())
	})

	r.GET("/logout", h.Logout)

	// API routes
	api := r.Group("/api")
	{
		api.POST("/login", h.Login)

		// Cluster management
		api.GET("/clusters", h.ListClusters)
		api.POST("/clusters", h.AddCluster)
		api.DELETE("/clusters/:id", h.DeleteCluster)

		// CoreDNS management
		api.GET("/clusters/:id/coredns", h.GetCoreDNSConfig)
		api.PUT("/clusters/:id/coredns", h.UpdateCorefile)
		api.POST("/clusters/:id/rules", h.AddForwardRule)
		api.DELETE("/clusters/:id/rules/:namespace", h.DeleteForwardRule)
	}

	// Start server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Starting CoreDNS Manager on http://%s", addr)
	log.Printf("Login with username: %s", cfg.Auth.Username)

	if err := r.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}

// render renders a templ component
func render(c *gin.Context, status int, template templ.Component) {
	c.Status(status)
	c.Header("Content-Type", "text/html; charset=utf-8")
	if err := template.Render(c.Request.Context(), c.Writer); err != nil {
		c.String(http.StatusInternalServerError, "Template rendering error")
	}
}
