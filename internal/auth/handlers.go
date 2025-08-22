package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// AuthHandlers contains the HTTP handlers for authentication
type AuthHandlers struct {
	authService *AuthService
	logger      *slog.Logger
}

// NewAuthHandlers creates new authentication handlers
func NewAuthHandlers(authService *AuthService, logger *slog.Logger) *AuthHandlers {
	return &AuthHandlers{
		authService: authService,
		logger:      logger,
	}
}

// LoginHandler initiates the OIDC authentication flow
func (h *AuthHandlers) LoginHandler(w http.ResponseWriter, r *http.Request) {
	// Check if user is already authenticated
	if _, err := h.authService.ValidateSession(r); err == nil {
		h.logger.Debug("User already authenticated, redirecting to home")
		http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
		return
	}

	// Generate auth URL and state
	authURL, state, err := h.authService.GetAuthURL()
	if err != nil {
		h.logger.Error("Failed to generate auth URL", "error", err)
		http.Error(w, "Authentication service unavailable", http.StatusInternalServerError)
		return
	}

	// Store state in session for verification
	session, err := h.authService.store.Get(r, "go-media-control-session")
	if err != nil {
		h.logger.Error("Failed to get session for state storage", "error", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug("Session before saving", "session_id", session.ID, "is_new", session.IsNew)

	// Clear any existing auth data
	session.Values["oauth_state"] = state
	session.Values["oauth_timestamp"] = time.Now().Unix()
	session.Values["authenticated"] = false

	if err := session.Save(r, w); err != nil {
		h.logger.Error("Failed to save session state", "error", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug("Stored OAuth state in session", "state", state, "session_id", session.ID)

	h.logger.Info("Redirecting user to authentication provider", "auth_url", authURL)
	http.Redirect(w, r, authURL, http.StatusTemporaryRedirect)
}

// CallbackHandler handles the OAuth2 callback from Authentik
func (h *AuthHandlers) CallbackHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Get authorization code and state from query parameters
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		h.logger.Error("No authorization code in callback")
		http.Error(w, "Authorization code missing", http.StatusBadRequest)
		return
	}

	if state == "" {
		h.logger.Error("No state parameter in callback")
		http.Error(w, "State parameter missing", http.StatusBadRequest)
		return
	}

	// Verify state parameter
	session, err := h.authService.store.Get(r, "go-media-control-session")
	if err != nil {
		h.logger.Error("Failed to get session for state verification", "error", err)
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	h.logger.Debug("Session during callback", "session_id", session.ID, "is_new", session.IsNew, "values", session.Values)

	storedState, ok := session.Values["oauth_state"].(string)
	if !ok {
		h.logger.Error("No stored state found in session", "session_values", session.Values)
		http.Error(w, "No stored state found", http.StatusBadRequest)
		return
	}

	if storedState != state {
		h.logger.Error("Invalid state parameter", "expected", storedState, "received", state)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	h.logger.Debug("State parameter verified successfully")

	// Check if state is not too old (5 minutes max)
	timestamp, ok := session.Values["oauth_timestamp"].(int64)
	if !ok || time.Now().Unix()-timestamp > 300 {
		h.logger.Error("State parameter expired")
		http.Error(w, "Authentication request expired", http.StatusBadRequest)
		return
	}

	// Exchange code for user info
	userInfo, err := h.authService.HandleCallback(ctx, code, state)
	if err != nil {
		h.logger.Error("Failed to handle OAuth callback", "error", err)
		http.Error(w, "Authentication failed", http.StatusInternalServerError)
		return
	}

	// Create session for authenticated user
	if err := h.authService.CreateSession(w, r, userInfo); err != nil {
		h.logger.Error("Failed to create user session", "error", err)
		http.Error(w, "Session creation failed", http.StatusInternalServerError)
		return
	}

	h.logger.Info("User successfully authenticated",
		"user_id", userInfo.Subject,
		"username", userInfo.PreferredUsername,
		"name", userInfo.Name)

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusTemporaryRedirect)
}

// LogoutHandler clears the user session and optionally redirects to Authentik logout
func (h *AuthHandlers) LogoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the session
	if err := h.authService.ClearSession(w, r); err != nil {
		h.logger.Error("Failed to clear session", "error", err)
		// Continue with logout even if session clearing fails
	}

	h.logger.Info("User logged out")

	// Check if we should redirect to Authentik logout
	logoutURL := fmt.Sprintf("%s/application/o/go-media-control/end-session/", h.authService.config.AuthentikURL)

	// You can optionally redirect to Authentik's logout endpoint
	// Check if we should redirect to Authentik logout
	redirectToAuthentik := r.URL.Query().Get("global") == "true"

	if redirectToAuthentik {
		// Add post logout redirect URL if needed
		baseURL := h.authService.config.RedirectURL[:len(h.authService.config.RedirectURL)-len("/auth/callback")]
		postLogoutURL := fmt.Sprintf("%s/auth/logged-out", baseURL)
		logoutURL += fmt.Sprintf("?post_logout_redirect_uri=%s", postLogoutURL)
		http.Redirect(w, r, logoutURL, http.StatusTemporaryRedirect)
		return
	}

	// Simple logout - just redirect to login page
	http.Redirect(w, r, "/auth/login", http.StatusTemporaryRedirect)
}

// LoggedOutHandler shows a simple logged out page
func (h *AuthHandlers) LoggedOutHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	html := `<!DOCTYPE html>
<html>
<head>
    <title>Logged Out - Go Media Control</title>
    <style>
        body { font-family: Arial, sans-serif; text-align: center; margin-top: 50px; }
        .container { max-width: 400px; margin: 0 auto; padding: 20px; }
        .message { color: #666; margin-bottom: 20px; }
        .btn { display: inline-block; padding: 10px 20px; background: #007bff; color: white; text-decoration: none; border-radius: 4px; }
        .btn:hover { background: #0056b3; }
    </style>
</head>
<body>
    <div class="container">
        <h1>Logged Out</h1>
        <p class="message">You have been successfully logged out.</p>
        <a href="/auth/login" class="btn">Log In Again</a>
    </div>
</body>
</html>`

	w.Write([]byte(html))
}

// UserInfoHandler returns current user information (for debugging/API)
func (h *AuthHandlers) UserInfoHandler(w http.ResponseWriter, r *http.Request) {
	userInfo, err := h.authService.ValidateSession(r)
	if err != nil {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	response := fmt.Sprintf(`{
		"subject": "%s",
		"username": "%s",
		"name": "%s",
		"email": "%s"
	}`, userInfo.Subject, userInfo.PreferredUsername, userInfo.Name, userInfo.Email)

	w.Write([]byte(response))
}
