package admin

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/opentide/opentide/internal/security"
)

const (
	cookieName     = "opentide_session"
	sessionMaxAge  = 24 * time.Hour
	loginRateKey   = "admin:login"
)

// loginRequest is the POST body for /admin/api/login.
type loginRequest struct {
	Secret string `json:"secret"`
}

// authMiddleware wraps handlers to require a valid session cookie.
// In demo mode, all requests are allowed through.
func (s *Server) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if s.config.Gateway.DemoMode {
			next(w, r)
			return
		}

		cookie, err := r.Cookie(cookieName)
		if err != nil {
			s.jsonError(w, "authentication required", http.StatusUnauthorized)
			return
		}

		if !s.validateSession(cookie.Value) {
			s.jsonError(w, "invalid or expired session", http.StatusUnauthorized)
			return
		}

		next(w, r)
	}
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)

	// Rate limit login attempts
	if s.rateLimiter != nil {
		ip := r.RemoteAddr
		key := security.RateLimitKey(loginRateKey, ip)
		if !s.rateLimiter.Allow(key) {
			s.jsonError(w, "too many login attempts, try again later", http.StatusTooManyRequests)
			return
		}
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.Secret), []byte(s.config.Security.AdminSecret)) != 1 {
		s.logger.Warn("failed login attempt", "remote_addr", r.RemoteAddr)
		s.jsonError(w, "invalid secret", http.StatusUnauthorized)
		return
	}

	token := s.createSession()
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    token,
		Path:     "/admin",
		MaxAge:   int(sessionMaxAge.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
		Secure:   r.TLS != nil,
	})

	s.logger.Info("admin login successful", "remote_addr", r.RemoteAddr)
	s.jsonOK(w, map[string]string{"status": "authenticated"})
}

func (s *Server) handleLogout(w http.ResponseWriter, _ *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    "",
		Path:     "/admin",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	s.jsonOK(w, map[string]string{"status": "logged_out"})
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if s.config.Gateway.DemoMode {
		s.jsonOK(w, map[string]any{"authenticated": true, "demo": true})
		return
	}

	cookie, err := r.Cookie(cookieName)
	if err != nil || !s.validateSession(cookie.Value) {
		s.jsonOK(w, map[string]any{"authenticated": false})
		return
	}
	s.jsonOK(w, map[string]any{"authenticated": true})
}

// createSession builds an HMAC-signed session token.
func (s *Server) createSession() string {
	payload := fmt.Sprintf("%d", time.Now().Unix())
	mac := s.computeMAC(payload)
	return payload + "." + mac
}

// validateSession checks that a session token is valid and not expired.
func (s *Server) validateSession(token string) bool {
	// Token format: "timestamp.hmac"
	var timestamp, mac string
	for i := len(token) - 1; i >= 0; i-- {
		if token[i] == '.' {
			timestamp = token[:i]
			mac = token[i+1:]
			break
		}
	}
	if timestamp == "" || mac == "" {
		return false
	}

	// Verify HMAC
	expectedMAC := s.computeMAC(timestamp)
	if !hmac.Equal([]byte(mac), []byte(expectedMAC)) {
		return false
	}

	// Check expiry
	ts, err := strconv.ParseInt(timestamp, 10, 64)
	if err != nil {
		return false
	}
	issuedAt := time.Unix(ts, 0)
	return time.Since(issuedAt) < sessionMaxAge
}

func (s *Server) computeMAC(data string) string {
	h := hmac.New(sha256.New, []byte(s.config.Security.AdminSecret))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}
