package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
)

const (
	cookieName    = "aradashboard_session"
	sessionLength = 32
)

// Store manages password authentication and session tokens.
type Store struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	ttl      time.Duration
	password string
}

// NewStore creates a new session store. If password is empty, auth is disabled.
func NewStore(password string, ttl time.Duration) *Store {
	return &Store{
		sessions: make(map[string]time.Time),
		ttl:      ttl,
		password: password,
	}
}

// Enabled returns true if a password is configured.
func (s *Store) Enabled() bool {
	return s.password != ""
}

// CheckPassword compares input against the configured password using constant-time comparison.
func (s *Store) CheckPassword(input string) bool {
	return subtle.ConstantTimeCompare([]byte(input), []byte(s.password)) == 1
}

// CreateSession generates a new session token and stores it.
func (s *Store) CreateSession() (string, error) {
	b := make([]byte, sessionLength)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)

	s.mu.Lock()
	s.sessions[token] = time.Now().Add(s.ttl)
	s.mu.Unlock()

	return token, nil
}

// ValidateSession checks if a session token is valid and not expired.
func (s *Store) ValidateSession(token string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	expiry, ok := s.sessions[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(s.sessions, token)
		return false
	}
	return true
}

// DestroySession removes a session token.
func (s *Store) DestroySession(token string) {
	s.mu.Lock()
	delete(s.sessions, token)
	s.mu.Unlock()
}

// Middleware returns an Echo middleware that enforces authentication.
// Requests to paths starting with any of the skip prefixes bypass auth.
func Middleware(store *Store, skipPrefixes ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			if !store.Enabled() {
				return next(c)
			}

			path := c.Request().URL.Path
			for _, prefix := range skipPrefixes {
				if path == prefix || strings.HasPrefix(path, prefix+"/") {
					return next(c)
				}
			}

			cookie, err := c.Cookie(cookieName)
			if err != nil || !store.ValidateSession(cookie.Value) {
				if strings.HasPrefix(path, "/api/") {
					return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
				}
				return c.Redirect(http.StatusFound, "/login")
			}

			return next(c)
		}
	}
}

// LoginHandler handles POST /login.
func LoginHandler(store *Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		password := c.FormValue("password")
		if !store.CheckPassword(password) {
			return c.Render(http.StatusOK, "login.html", map[string]interface{}{
				"Error": "Invalid password",
			})
		}

		token, err := store.CreateSession()
		if err != nil {
			return c.Render(http.StatusInternalServerError, "login.html", map[string]interface{}{
				"Error": "Internal error",
			})
		}

		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   int(store.ttl.Seconds()),
		})

		return c.Redirect(http.StatusFound, "/")
	}
}

// LoginPageHandler handles GET /login.
func LoginPageHandler(store *Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !store.Enabled() {
			return c.Redirect(http.StatusFound, "/")
		}
		// Already logged in?
		if cookie, err := c.Cookie(cookieName); err == nil && store.ValidateSession(cookie.Value) {
			return c.Redirect(http.StatusFound, "/")
		}
		return c.Render(http.StatusOK, "login.html", map[string]interface{}{})
	}
}

// LogoutHandler handles GET /logout.
func LogoutHandler(store *Store) echo.HandlerFunc {
	return func(c echo.Context) error {
		if cookie, err := c.Cookie(cookieName); err == nil {
			store.DestroySession(cookie.Value)
		}
		c.SetCookie(&http.Cookie{
			Name:     cookieName,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			MaxAge:   -1,
		})
		return c.Redirect(http.StatusFound, "/login")
	}
}
