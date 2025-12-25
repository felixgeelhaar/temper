package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/felixgeelhaar/temper/internal/auth"
)

// AuthHandler handles authentication endpoints
type AuthHandler struct {
	authService *auth.Service
	cookieName  string
	cookieMaxAge int
	secureCookie bool
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler(authService *auth.Service, secureCookie bool, maxAge int) *AuthHandler {
	return &AuthHandler{
		authService:  authService,
		cookieName:   "session",
		cookieMaxAge: maxAge,
		secureCookie: secureCookie,
	}
}

// RegisterRequest is the request body for registration
type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

// LoginRequest is the request body for login
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// UserResponse is the response for user data
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

// Register handles user registration
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if req.Email == "" || req.Password == "" {
		h.jsonError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	if len(req.Password) < 8 {
		h.jsonError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	user, err := h.authService.Register(r.Context(), auth.RegisterRequest{
		Email:    req.Email,
		Name:     req.Name,
		Password: req.Password,
	})

	if errors.Is(err, auth.ErrEmailExists) {
		h.jsonError(w, http.StatusConflict, "email already registered")
		return
	}
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "registration failed")
		return
	}

	h.jsonResponse(w, http.StatusCreated, map[string]any{
		"user": UserResponse{
			ID:        user.ID.String(),
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
	})
}

// Login handles user login
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Email == "" || req.Password == "" {
		h.jsonError(w, http.StatusBadRequest, "email and password are required")
		return
	}

	result, err := h.authService.Login(r.Context(), auth.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	})

	if errors.Is(err, auth.ErrInvalidCredentials) {
		h.jsonError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		h.jsonError(w, http.StatusInternalServerError, "login failed")
		return
	}

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    result.Token,
		Path:     "/",
		MaxAge:   h.cookieMaxAge,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"user": UserResponse{
			ID:        result.User.ID.String(),
			Email:     result.User.Email,
			Name:      result.User.Name,
			CreatedAt: result.User.CreatedAt.Format(time.RFC3339),
		},
		"token": result.Token,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		h.jsonError(w, http.StatusBadRequest, "not logged in")
		return
	}

	if err := h.authService.Logout(r.Context(), cookie.Value); err != nil {
		// Log but don't fail - user wants to log out anyway
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     h.cookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	h.jsonResponse(w, http.StatusOK, map[string]string{
		"message": "logged out successfully",
	})
}

// Me returns the current user
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cookieName)
	if err != nil {
		h.jsonError(w, http.StatusUnauthorized, "not authenticated")
		return
	}

	user, _, err := h.authService.ValidateSession(r.Context(), cookie.Value)
	if err != nil {
		// Clear invalid cookie
		http.SetCookie(w, &http.Cookie{
			Name:     h.cookieName,
			Value:    "",
			Path:     "/",
			MaxAge:   -1,
			HttpOnly: true,
		})
		h.jsonError(w, http.StatusUnauthorized, "session expired")
		return
	}

	h.jsonResponse(w, http.StatusOK, map[string]any{
		"user": UserResponse{
			ID:        user.ID.String(),
			Email:     user.Email,
			Name:      user.Name,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
	})
}

func (h *AuthHandler) jsonResponse(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *AuthHandler) jsonError(w http.ResponseWriter, status int, message string) {
	h.jsonResponse(w, status, map[string]string{"error": message})
}
