package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"coredns-multi-configuration/pkg/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrInvalidToken       = errors.New("invalid or expired token")
)

// Claims represents JWT claims
type Claims struct {
	Username string `json:"username"`
	jwt.RegisteredClaims
}

// Auth handles authentication operations
type Auth struct {
	config *config.AuthConfig
}

// New creates a new Auth instance
func New(cfg *config.AuthConfig) *Auth {
	return &Auth{config: cfg}
}

// ValidateCredentials validates username and password
func (a *Auth) ValidateCredentials(username, password string) error {
	if username == a.config.Username && password == a.config.Password {
		return nil
	}
	return ErrInvalidCredentials
}

// GenerateToken generates a JWT token for the user
func (a *Auth) GenerateToken(username string) (string, error) {
	claims := &Claims{
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   username,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.config.JWTSecret))
}

// ValidateToken validates a JWT token and returns the claims
func (a *Auth) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(a.config.JWTSecret), nil
	})

	if err != nil {
		return nil, ErrInvalidToken
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// Middleware returns a Gin middleware for authentication
func (a *Auth) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip login and static routes
		if c.Request.URL.Path == "/login" ||
			c.Request.URL.Path == "/api/login" ||
			strings.HasPrefix(c.Request.URL.Path, "/static/") {
			c.Next()
			return
		}

		// Check for token in cookie first
		tokenString, err := c.Cookie("token")
		if err != nil {
			// Try Authorization header
			authHeader := c.GetHeader("Authorization")
			if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				tokenString = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}

		if tokenString == "" {
			// Redirect to login for page requests
			if c.Request.Header.Get("HX-Request") == "" && !strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.Redirect(http.StatusTemporaryRedirect, "/login")
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}

		claims, err := a.ValidateToken(tokenString)
		if err != nil {
			if c.Request.Header.Get("HX-Request") == "" && !strings.HasPrefix(c.Request.URL.Path, "/api/") {
				c.Redirect(http.StatusTemporaryRedirect, "/login")
				c.Abort()
				return
			}
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			c.Abort()
			return
		}

		c.Set("username", claims.Username)
		c.Next()
	}
}
