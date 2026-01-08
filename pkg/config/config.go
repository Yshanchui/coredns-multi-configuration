package config

import (
	"os"

	"github.com/goccy/go-yaml"
)

// Config represents the application configuration
type Config struct {
	Server   ServerConfig `yaml:"server"`
	Auth     AuthConfig   `yaml:"auth"`
	DataDir  string       `yaml:"data_dir"`
	LogLevel string       `yaml:"log_level"`
}

// ServerConfig represents HTTP server configuration
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Username  string `yaml:"username"`
	Password  string `yaml:"password"`
	JWTSecret string `yaml:"jwt_secret"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "0.0.0.0",
			Port: 8080,
		},
		Auth: AuthConfig{
			Username:  "admin",
			Password:  "admin123",
			JWTSecret: "coredns-manager-secret-key-change-me",
		},
		DataDir:  "./data",
		LogLevel: "info",
	}
}

// Load loads configuration from a YAML file
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return default config if file doesn't exist
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Override with environment variables
	applyEnvOverrides(cfg)

	return cfg, nil
}

// applyEnvOverrides overrides configuration with environment variables
func applyEnvOverrides(cfg *Config) {
	if username := os.Getenv("AUTH_USERNAME"); username != "" {
		cfg.Auth.Username = username
	}
	if password := os.Getenv("AUTH_PASSWORD"); password != "" {
		cfg.Auth.Password = password
	}
	if jwtSecret := os.Getenv("AUTH_JWT_SECRET"); jwtSecret != "" {
		cfg.Auth.JWTSecret = jwtSecret
	}
}

// Save saves configuration to a YAML file
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
