package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/git-saj/go-media-control/internal/config"
	"github.com/gorilla/sessions"
	"golang.org/x/oauth2"
)

// AuthService handles OIDC authentication with Authentik
type AuthService struct {
	config       *config.Config
	logger       *slog.Logger
	provider     *oidc.Provider
	oauth2Config oauth2.Config
	store        *sessions.CookieStore
	basePath     string
}

// UserInfo contains basic user information from OIDC
type UserInfo struct {
	Subject           string `json:"sub"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	Email             string `json:"email"`
}

// NewAuthService creates a new authentication service
func NewAuthService(cfg *config.Config, logger *slog.Logger) (*AuthService, error) {
	ctx := context.Background()

	// Discover OIDC provider configuration
	baseURL := strings.TrimSuffix(cfg.AuthentikURL, "/")
	providerURL := fmt.Sprintf("%s/application/o/go-media-control/", baseURL)
	logger.Info("Initializing OIDC provider", "provider_url", providerURL)
	provider, err := oidc.NewProvider(ctx, providerURL)
	if err != nil {
		logger.Error("Failed to initialize OIDC provider", "provider_url", providerURL, "error", err)
		return nil, fmt.Errorf("failed to get OIDC provider at %s: %w", providerURL, err)
	}
	logger.Info("Successfully initialized OIDC provider")

	// Configure OAuth2
	oauth2Config := oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Endpoint:     provider.Endpoint(),
		Scopes:       []string{oidc.ScopeOpenID, "profile", "email"},
	}

	// Create secure cookie store
	store := sessions.NewCookieStore([]byte(cfg.SessionSecret))
	// Determine if we're in development mode (no HTTPS)
	isProduction := len(cfg.RedirectURL) > 5 && cfg.RedirectURL[:5] == "https"

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   24 * 60 * 60, // 24 hours
		HttpOnly: true,
		Secure:   isProduction,         // Only secure cookies in production (HTTPS)
		SameSite: http.SameSiteLaxMode, // Changed from Strict to Lax for OAuth callbacks
		Domain:   "",                   // Allow cookies across subdomains if needed
	}

	return &AuthService{
		config:       cfg,
		logger:       logger,
		provider:     provider,
		oauth2Config: oauth2Config,
		store:        store,
		basePath:     cfg.BasePath,
	}, nil
}

// generateRandomState generates a random state parameter for OAuth2 security
func (a *AuthService) generateRandomState() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// GetAuthURL returns the URL to redirect users to for authentication
func (a *AuthService) GetAuthURL() (string, string, error) {
	state, err := a.generateRandomState()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate state: %w", err)
	}

	url := a.oauth2Config.AuthCodeURL(state)
	return url, state, nil
}

// HandleCallback processes the OAuth2 callback and returns user information
func (a *AuthService) HandleCallback(ctx context.Context, code, state string) (*UserInfo, error) {
	// Exchange code for tokens
	token, err := a.oauth2Config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange code: %w", err)
	}

	// Extract ID token
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, fmt.Errorf("no id_token in token response")
	}

	// Verify ID token
	verifier := a.provider.Verifier(&oidc.Config{
		ClientID: a.config.ClientID,
	})

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("failed to verify ID token: %w", err)
	}

	// Extract user info from ID token
	var userInfo UserInfo
	if err := idToken.Claims(&userInfo); err != nil {
		return nil, fmt.Errorf("failed to parse user info: %w", err)
	}

	return &userInfo, nil
}

// CreateSession creates a secure session for the authenticated user
func (a *AuthService) CreateSession(w http.ResponseWriter, r *http.Request, userInfo *UserInfo) error {
	session, err := a.store.Get(r, "go-media-control-session")
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.Values["user_id"] = userInfo.Subject
	session.Values["username"] = userInfo.PreferredUsername
	session.Values["name"] = userInfo.Name
	session.Values["email"] = userInfo.Email
	session.Values["authenticated"] = true

	return session.Save(r, w)
}

// ValidateSession validates a session and returns user info
func (a *AuthService) ValidateSession(r *http.Request) (*UserInfo, error) {
	session, err := a.store.Get(r, "go-media-control-session")
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	authenticated, ok := session.Values["authenticated"].(bool)
	if !ok || !authenticated {
		return nil, fmt.Errorf("not authenticated")
	}

	userID, ok := session.Values["user_id"].(string)
	if !ok {
		return nil, fmt.Errorf("no user ID in session")
	}

	username, _ := session.Values["username"].(string)
	name, _ := session.Values["name"].(string)
	email, _ := session.Values["email"].(string)

	return &UserInfo{
		Subject:           userID,
		PreferredUsername: username,
		Name:              name,
		Email:             email,
	}, nil
}

// ClearSession removes the session
func (a *AuthService) ClearSession(w http.ResponseWriter, r *http.Request) error {
	session, err := a.store.Get(r, "go-media-control-session")
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}

	session.Values["authenticated"] = false
	session.Options.MaxAge = -1 // Delete immediately

	return session.Save(r, w)
}

// RequireAuth is middleware that ensures the user is authenticated
func (a *AuthService) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for login/callback endpoints
		authLoginPath := strings.TrimSuffix(a.basePath, "/") + "/auth/login"
		authCallbackPath := strings.TrimSuffix(a.basePath, "/") + "/auth/callback"
		if r.URL.Path == authLoginPath || r.URL.Path == authCallbackPath {
			next.ServeHTTP(w, r)
			return
		}

		userInfo, err := a.ValidateSession(r)
		if err != nil {
			a.logger.Debug("Authentication required", "error", err, "path", r.URL.Path)
			loginURL := strings.TrimSuffix(a.basePath, "/") + "/auth/login"
			http.Redirect(w, r, loginURL, http.StatusTemporaryRedirect)
			return
		}

		// Add user info to request context
		ctx := context.WithValue(r.Context(), "user", userInfo)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetUserFromContext extracts user info from request context
func GetUserFromContext(ctx context.Context) (*UserInfo, bool) {
	user, ok := ctx.Value("user").(*UserInfo)
	return user, ok
}
