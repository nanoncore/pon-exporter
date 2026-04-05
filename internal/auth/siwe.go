package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	siwe "github.com/spruceid/siwe-go"
)

//go:embed login.html
var loginFS embed.FS

// Config holds WalletConnect auth configuration.
type Config struct {
	ProjectID        string   `yaml:"project_id"`
	AllowedAddresses []string `yaml:"allowed_addresses,omitempty"`
	SessionTTL       Duration `yaml:"session_ttl,omitempty"`
}

// Duration wraps time.Duration for YAML.
type Duration time.Duration

func (d *Duration) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	*d = Duration(dur)
	return nil
}

// Middleware wraps an http.Handler with SIWE authentication.
type Middleware struct {
	next             http.Handler
	projectID        string
	allowedAddresses map[string]bool
	sessionTTL       time.Duration
	hmacKey          []byte
	nonces           map[string]time.Time
	mu               sync.Mutex
	logger           *slog.Logger
}

// New creates auth middleware. Returns the inner handler unwrapped if cfg is nil.
func New(next http.Handler, cfg *Config, logger *slog.Logger) http.Handler {
	if cfg == nil {
		return next
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic("failed to generate HMAC key: " + err.Error())
	}

	ttl := time.Duration(cfg.SessionTTL)
	if ttl == 0 {
		ttl = 24 * time.Hour
	}

	allowed := make(map[string]bool, len(cfg.AllowedAddresses))
	for _, addr := range cfg.AllowedAddresses {
		allowed[strings.ToLower(addr)] = true
	}

	m := &Middleware{
		next:             next,
		projectID:        cfg.ProjectID,
		allowedAddresses: allowed,
		sessionTTL:       ttl,
		hmacKey:          key,
		nonces:           make(map[string]time.Time),
		logger:           logger,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/login", m.handleLogin)
	mux.HandleFunc("/auth/nonce", m.handleNonce)
	mux.HandleFunc("/auth/verify", m.handleVerify)
	mux.HandleFunc("/auth/logout", m.handleLogout)
	mux.Handle("/", m)

	return mux
}

func (m *Middleware) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Health and readiness endpoints bypass auth.
	if r.URL.Path == "/-/healthy" || r.URL.Path == "/-/ready" {
		m.next.ServeHTTP(w, r)
		return
	}

	cookie, err := r.Cookie("pon_session")
	if err != nil || !m.validSession(cookie.Value) {
		http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
		return
	}
	m.next.ServeHTTP(w, r)
}

func (m *Middleware) handleLogin(w http.ResponseWriter, _ *http.Request) {
	html, _ := loginFS.ReadFile("login.html")
	content := strings.ReplaceAll(string(html), "{{PROJECT_ID}}", m.projectID)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, content)
}

func (m *Middleware) handleNonce(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nonce := generateNonce()

	m.mu.Lock()
	m.nonces[nonce] = time.Now().Add(5 * time.Minute)
	// Clean expired nonces.
	for k, exp := range m.nonces {
		if time.Now().After(exp) {
			delete(m.nonces, k)
		}
	}
	m.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, nonce)
}

func (m *Middleware) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	rawMessage := r.FormValue("message")
	signature := r.FormValue("signature")

	msg, err := siwe.ParseMessage(rawMessage)
	if err != nil {
		m.logger.Warn("SIWE parse failed", "err", err)
		http.Error(w, "invalid SIWE message", http.StatusBadRequest)
		return
	}

	// Verify nonce is valid and unused.
	nonce := msg.GetNonce()
	m.mu.Lock()
	exp, ok := m.nonces[nonce]
	if ok {
		delete(m.nonces, nonce)
	}
	m.mu.Unlock()

	if !ok || time.Now().After(exp) {
		http.Error(w, "invalid or expired nonce", http.StatusUnauthorized)
		return
	}

	// Verify signature.
	_, err = msg.Verify(signature, nil, nil, nil)
	if err != nil {
		m.logger.Warn("SIWE verify failed", "err", err)
		http.Error(w, "signature verification failed", http.StatusUnauthorized)
		return
	}

	address := strings.ToLower(msg.GetAddress().Hex())

	// Check allowlist if configured.
	if len(m.allowedAddresses) > 0 && !m.allowedAddresses[address] {
		m.logger.Warn("address not in allowlist", "address", address)
		http.Error(w, "address not authorized", http.StatusForbidden)
		return
	}

	m.logger.Info("wallet authenticated", "address", address)

	// Set session cookie.
	session := m.signSession(address)
	http.SetCookie(w, &http.Cookie{
		Name:     "pon_session",
		Value:    session,
		Path:     "/",
		MaxAge:   int(m.sessionTTL.Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "ok")
}

func (m *Middleware) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:   "pon_session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
	http.Redirect(w, r, "/auth/login", http.StatusSeeOther)
}

func (m *Middleware) signSession(address string) string {
	expiry := time.Now().Add(m.sessionTTL).Unix()
	payload := fmt.Sprintf("%s:%d", address, expiry)
	mac := hmac.New(sha256.New, m.hmacKey)
	mac.Write([]byte(payload))
	sig := hex.EncodeToString(mac.Sum(nil))
	return fmt.Sprintf("%s.%s", payload, sig)
}

func (m *Middleware) validSession(value string) bool {
	parts := strings.SplitN(value, ".", 2)
	if len(parts) != 2 {
		return false
	}
	payload, sig := parts[0], parts[1]

	mac := hmac.New(sha256.New, m.hmacKey)
	mac.Write([]byte(payload))
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return false
	}

	// Check expiry.
	var addr string
	var expiry int64
	if _, err := fmt.Sscanf(payload, "%s", &addr); err != nil {
		return false
	}
	idx := strings.LastIndex(payload, ":")
	if idx < 0 {
		return false
	}
	if _, err := fmt.Sscanf(payload[idx+1:], "%d", &expiry); err != nil {
		return false
	}

	return time.Now().Unix() < expiry
}

func generateNonce() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("failed to generate nonce: " + err.Error())
	}
	return hex.EncodeToString(b)
}
