package handler

import (
	"context"
	"errors"
	"log"

	auth "github.com/RishangS/auth-service/gen/proto"
	"github.com/RishangS/auth-service/utils"
)

type AuthHandler struct {
	auth.UnimplementedAuthServiceServer
	userRepo   *utils.UserRepository
	authClient *utils.AuthClient
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		userRepo:   utils.NewDBService(),
		authClient: utils.NewAuthClient(),
	}
}

// Signup handles user registration
func (h *AuthHandler) Signup(ctx context.Context, req *auth.SignupRequest) (*auth.SignupResponse, error) {
	if req.Username == "" || req.Password == "" || req.Email == "" {
		return nil, errors.New("username, password and email are required")
	}

	user, err := h.userRepo.CreateUser(ctx, req.Username, req.Password, req.Email)
	if err != nil {
		return nil, err
	}

	return &auth.SignupResponse{
		UserId:   int64(user.ID),
		Username: user.Username,
		Email:    user.Email,
	}, nil
}

// Login handles user authentication and returns JWT tokens
func (h *AuthHandler) Login(ctx context.Context, req *auth.LoginRequest) (*auth.LoginResponse, error) {
	if req.Username == "" || req.Password == "" {
		return nil, errors.New("username and password are required")
	}

	// Authenticate user
	user, err := h.userRepo.AuthenticateUser(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}

	// Generate access token
	accessToken, err := h.authClient.GenerateJWT(user.ID)
	if err != nil {
		return nil, err
	}

	// Generate refresh token
	refreshToken, err := h.authClient.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	return &auth.LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
	}, nil
}

// VerifyToken validates the JWT token and returns user information
func (h *AuthHandler) VerifyToken(ctx context.Context, req *auth.VerifyRequest) (*auth.VerifyResponse, error) {
	log.Println("VerifyToken")
	if req.Token == "" {
		return nil, errors.New("token is required")
	}

	claims, err := h.authClient.ValidateJWT(req.Token)
	if err != nil {
		return &auth.VerifyResponse{
			Valid: false,
		}, nil
	}

	// Get user ID from claims
	userID, ok := claims["user_id"].(float64)
	if !ok {
		return &auth.VerifyResponse{
			Valid: false,
		}, nil
	}

	// Get user details from database
	user, err := h.userRepo.GetUserByID(ctx, int(userID))
	if err != nil {
		return &auth.VerifyResponse{
			Valid: false,
		}, nil
	}

	return &auth.VerifyResponse{
		Valid:    true,
		Username: user.Username,
	}, nil
}

// RefreshToken generates a new access token using a valid refresh token
func (h *AuthHandler) RefreshToken(ctx context.Context, req *auth.RefreshRequest) (*auth.LoginResponse, error) {
	if req.RefreshToken == "" {
		return nil, errors.New("refresh token is required")
	}

	// Generate new access token using refresh token
	newAccessToken, err := h.authClient.RefreshAccessToken(req.RefreshToken)
	if err != nil {
		return nil, err
	}

	// Generate new refresh token
	claims, err := h.authClient.ValidateJWT(req.RefreshToken)
	if err != nil {
		return nil, err
	}

	userID, ok := claims["user_id"].(float64)
	if !ok {
		return nil, errors.New("invalid refresh token claims")
	}

	newRefreshToken, err := h.authClient.GenerateRefreshToken(int(userID))
	if err != nil {
		return nil, err
	}

	// Revoke old refresh token
	err = h.authClient.RevokeRefreshToken(req.RefreshToken)
	if err != nil {
		log.Printf("Warning: Failed to revoke old refresh token: %v", err)
	}

	return &auth.LoginResponse{
		AccessToken:  newAccessToken,
		RefreshToken: newRefreshToken,
	}, nil
}
