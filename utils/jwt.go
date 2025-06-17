package utils

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

type AuthClient struct {
	jwtSecret         string
	refreshTokenStore map[string]int // In-memory store for demo (use DB in production)
}

func NewAuthClient() *AuthClient {
	jwtSecret, ok := os.LookupEnv("JWT_SECRET")
	if jwtSecret == "" || !ok {
		log.Fatal("JWT Secret not found in env variables")
	}
	return &AuthClient{
		jwtSecret:         jwtSecret,
		refreshTokenStore: make(map[string]int), // Initialize the store
	}
}

// GenerateJWT generates a JWT token for a given user ID
func (a *AuthClient) GenerateJWT(userID int) (string, error) {
	claims := jwt.MapClaims{
		"user_id": userID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(), // Token expires after 24 hours
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Ensure jwtSecret is converted to []byte
	secretKey := []byte(a.jwtSecret)
	return token.SignedString(secretKey)
}

// GenerateRefreshToken generates a long-lived refresh token and stores it
func (a *AuthClient) GenerateRefreshToken(userID int) (string, error) {
	// Create refresh token with longer expiration (e.g., 7 days)
	claims := jwt.MapClaims{
		"user_id":    userID,
		"exp":        time.Now().Add(time.Hour * 24 * 7).Unix(), // 7 days expiration
		"is_refresh": true,                                      // Mark this as a refresh token
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token
	secretKey := []byte(a.jwtSecret)
	refreshToken, err := token.SignedString(secretKey)
	if err != nil {
		return "", err
	}

	// Store the refresh token (in-memory for this example)
	a.refreshTokenStore[refreshToken] = userID

	return refreshToken, nil
}

// RefreshAccessToken generates a new access token using a valid refresh token
func (a *AuthClient) RefreshAccessToken(refreshToken string) (string, error) {
	// First validate the refresh token
	claims, err := a.ValidateJWT(refreshToken)
	if err != nil {
		return "", fmt.Errorf("invalid refresh token: %v", err)
	}

	// Check if this is actually a refresh token
	if isRefresh, ok := claims["is_refresh"].(bool); !ok || !isRefresh {
		return "", fmt.Errorf("not a refresh token")
	}

	// Verify the refresh token exists in our store
	userID, exists := a.refreshTokenStore[refreshToken]
	if !exists {
		return "", fmt.Errorf("refresh token not found or revoked")
	}

	// Generate new access token
	newAccessToken, err := a.GenerateJWT(userID)
	if err != nil {
		return "", fmt.Errorf("failed to generate new access token: %v", err)
	}

	return newAccessToken, nil
}

// RevokeRefreshToken removes a refresh token from the store
func (a *AuthClient) RevokeRefreshToken(refreshToken string) error {
	if _, exists := a.refreshTokenStore[refreshToken]; !exists {
		return fmt.Errorf("refresh token not found")
	}
	delete(a.refreshTokenStore, refreshToken)
	return nil
}

// ValidateJWT validates the JWT token and returns the claims
func (a *AuthClient) ValidateJWT(tokenString string) (jwt.MapClaims, error) {
	// Parse the token
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Ensure the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(a.jwtSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %v", err)
	}

	// Extract claims if the token is valid
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token or claims")
}
