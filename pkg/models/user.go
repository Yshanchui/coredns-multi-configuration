package models

// User represents a user account for authentication
type User struct {
	Username string `json:"username"`
	Password string `json:"password"` // hashed password
}

// LoginRequest represents the login request body
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse represents the login response
type LoginResponse struct {
	Token   string `json:"token"`
	Message string `json:"message"`
}
