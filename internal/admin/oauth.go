package admin

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	googleAuthURL  = "https://accounts.google.com/o/oauth2/v2/auth"
	googleTokenURL = "https://oauth2.googleapis.com/token"
	googleUserURL  = "https://www.googleapis.com/oauth2/v2/userinfo"
	stateMaxAge    = 10 * time.Minute
)

// oauthStateStore holds pending OAuth states to prevent CSRF.
type oauthStateStore struct {
	mu     sync.Mutex
	states map[string]time.Time
}

func newOAuthStateStore() *oauthStateStore {
	return &oauthStateStore{states: make(map[string]time.Time)}
}

func (s *oauthStateStore) generate() string {
	b := make([]byte, 16)
	rand.Read(b)
	state := hex.EncodeToString(b)

	s.mu.Lock()
	s.states[state] = time.Now()
	// Cleanup old states
	for k, t := range s.states {
		if time.Since(t) > stateMaxAge {
			delete(s.states, k)
		}
	}
	s.mu.Unlock()
	return state
}

func (s *oauthStateStore) validate(state string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.states[state]
	if !ok {
		return false
	}
	delete(s.states, state)
	return time.Since(t) < stateMaxAge
}

// googleUserInfo is the response from the Google userinfo API.
type googleUserInfo struct {
	Email         string `json:"email"`
	VerifiedEmail bool   `json:"verified_email"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}

// handleGoogleLogin redirects the user to Google's OAuth consent screen.
func (s *Server) handleGoogleLogin(w http.ResponseWriter, r *http.Request) {
	cfg := s.config.Security
	if cfg.GoogleClientID == "" || cfg.GoogleClientSecret == "" {
		s.jsonError(w, "Google OAuth not configured", http.StatusNotFound)
		return
	}

	state := s.oauthStates.generate()

	// Build the redirect URI from the current request
	redirectURI := s.googleRedirectURI(r)

	params := url.Values{
		"client_id":     {cfg.GoogleClientID},
		"redirect_uri":  {redirectURI},
		"response_type": {"code"},
		"scope":         {"email profile"},
		"state":         {state},
		"prompt":        {"select_account"},
	}

	http.Redirect(w, r, googleAuthURL+"?"+params.Encode(), http.StatusTemporaryRedirect)
}

// handleGoogleCallback handles the OAuth callback from Google.
func (s *Server) handleGoogleCallback(w http.ResponseWriter, r *http.Request) {
	cfg := s.config.Security

	// Verify state parameter (CSRF protection)
	state := r.URL.Query().Get("state")
	if !s.oauthStates.validate(state) {
		http.Redirect(w, r, "/admin?error=invalid_state", http.StatusTemporaryRedirect)
		return
	}

	// Check for errors from Google
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		http.Redirect(w, r, "/admin?error="+url.QueryEscape(errMsg), http.StatusTemporaryRedirect)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Redirect(w, r, "/admin?error=missing_code", http.StatusTemporaryRedirect)
		return
	}

	redirectURI := s.googleRedirectURI(r)

	// Exchange code for token
	tokenResp, err := http.PostForm(googleTokenURL, url.Values{
		"code":          {code},
		"client_id":     {cfg.GoogleClientID},
		"client_secret": {cfg.GoogleClientSecret},
		"redirect_uri":  {redirectURI},
		"grant_type":    {"authorization_code"},
	})
	if err != nil {
		s.logger.Error("google token exchange failed", "err", err)
		http.Redirect(w, r, "/admin?error=token_exchange_failed", http.StatusTemporaryRedirect)
		return
	}
	defer tokenResp.Body.Close()

	var tokenData struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(tokenResp.Body).Decode(&tokenData); err != nil || tokenData.AccessToken == "" {
		s.logger.Error("google token response invalid", "err", err, "error_field", tokenData.Error)
		http.Redirect(w, r, "/admin?error=invalid_token", http.StatusTemporaryRedirect)
		return
	}

	// Get user info
	req, _ := http.NewRequestWithContext(r.Context(), "GET", googleUserURL, nil)
	req.Header.Set("Authorization", "Bearer "+tokenData.AccessToken)
	userResp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.logger.Error("google userinfo request failed", "err", err)
		http.Redirect(w, r, "/admin?error=userinfo_failed", http.StatusTemporaryRedirect)
		return
	}
	defer userResp.Body.Close()

	var userInfo googleUserInfo
	if err := json.NewDecoder(userResp.Body).Decode(&userInfo); err != nil {
		s.logger.Error("google userinfo decode failed", "err", err)
		http.Redirect(w, r, "/admin?error=userinfo_decode_failed", http.StatusTemporaryRedirect)
		return
	}

	if !userInfo.VerifiedEmail {
		s.logger.Warn("google login rejected: unverified email", "email", userInfo.Email)
		http.Redirect(w, r, "/admin?error=unverified_email", http.StatusTemporaryRedirect)
		return
	}

	// Check email allowlist
	if !s.isAllowedEmail(userInfo.Email) {
		s.logger.Warn("google login rejected: email not in allowlist", "email", userInfo.Email)
		http.Redirect(w, r, "/admin?error=unauthorized_email", http.StatusTemporaryRedirect)
		return
	}

	// Create session
	token := s.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/admin",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode, // Lax required for OAuth redirect
		Secure:   isSecureRequest(r),
	})

	s.logger.Info("google login successful", "email", userInfo.Email, "remote_addr", r.RemoteAddr)
	http.Redirect(w, r, "/admin", http.StatusTemporaryRedirect)
}

// handleAuthConfig returns the auth configuration so the frontend knows what login options are available.
func (s *Server) handleAuthConfig(w http.ResponseWriter, _ *http.Request) {
	cfg := s.config.Security
	s.jsonOK(w, map[string]any{
		"google_enabled": cfg.GoogleClientID != "" && cfg.GoogleClientSecret != "",
		"secret_enabled": cfg.AdminSecret != "",
		"demo_mode":      s.config.Gateway.DemoMode,
	})
}

// isAllowedEmail checks if an email is in the admin allowlist.
// If no allowlist is configured, all verified Google emails are allowed.
func (s *Server) isAllowedEmail(email string) bool {
	if s.config.Security.AdminEmails == "" {
		return true
	}
	email = strings.ToLower(strings.TrimSpace(email))
	for _, allowed := range strings.Split(s.config.Security.AdminEmails, ",") {
		if strings.ToLower(strings.TrimSpace(allowed)) == email {
			return true
		}
	}
	return false
}

// googleRedirectURI builds the OAuth callback URI from the request.
func (s *Server) googleRedirectURI(r *http.Request) string {
	scheme := "https"
	if r.TLS == nil && r.Header.Get("X-Forwarded-Proto") != "https" {
		scheme = "http"
	}
	host := r.Host
	return fmt.Sprintf("%s://%s/admin/api/auth/google/callback", scheme, host)
}
