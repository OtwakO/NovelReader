package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	SessionCookieName   = "novelreader_session"
	maxLoginRequestSize = 4 * 1024
	maxLoginRatePeers   = 4096
)

type HTTPConfig struct {
	PublicURL              string
	LoginTimeout           time.Duration
	LoginAttempts          int
	LoginWindow            time.Duration
	Readers                *readerstore.Manager
	RegistrationEnabled    bool
	RegistrationInviteCode string
}

type HTTPHandler struct {
	accounts           *AccountService
	sessions           *SessionService
	mux                *http.ServeMux
	allowedOrigin      string
	secureCookie       bool
	loginTimeout       time.Duration
	loginLimiter       *loginRateLimiter
	now                func() time.Time
	afterSessionCreate func(SessionCredential)
	registration       *RegistrationService
	registerAccount    func(context.Context, string, string, string, time.Time) (Account, error)
}

type loginRateLimiter struct {
	mutex    sync.Mutex
	attempts map[string]loginAttemptWindow
	limit    int
	window   time.Duration
}

type loginAttemptWindow struct {
	count int
	start time.Time
}

type identityContextKey struct{}

type registrationPolicyResponse struct {
	Enabled        bool `json:"enabled"`
	InviteRequired bool `json:"inviteRequired"`
}

type registrationRequest struct {
	Username   string
	Password   string
	InviteCode string
}

type accountResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}

func NewHTTPHandler(store *Store, config HTTPConfig) (*HTTPHandler, error) {
	origin, secure, err := ParsePublicOrigin(config.PublicURL)
	if err != nil {
		return nil, err
	}
	if config.LoginTimeout <= 0 {
		config.LoginTimeout = 5 * time.Second
	}
	if config.LoginAttempts <= 0 {
		config.LoginAttempts = 10
	}
	if config.LoginWindow <= 0 {
		config.LoginWindow = time.Minute
	}
	handler := &HTTPHandler{
		accounts:      NewAccountService(store),
		sessions:      NewSessionService(store),
		mux:           http.NewServeMux(),
		allowedOrigin: origin,
		secureCookie:  secure,
		loginTimeout:  config.LoginTimeout,
		loginLimiter: &loginRateLimiter{
			attempts: make(map[string]loginAttemptWindow),
			limit:    config.LoginAttempts,
			window:   config.LoginWindow,
		},
		now: time.Now,
	}
	if config.Readers != nil {
		handler.registration = NewRegistrationService(store, config.Readers, config.RegistrationEnabled, config.RegistrationInviteCode)
		handler.registerAccount = handler.registration.Register
	}
	handler.mux.HandleFunc("POST /api/auth/login", handler.handleLogin)
	handler.mux.HandleFunc("POST /api/auth/logout", handler.handleLogout)
	handler.mux.HandleFunc("GET /api/auth/registration", handler.handleRegistrationPolicy)
	handler.mux.HandleFunc("POST /api/auth/register", handler.handleRegister)
	handler.mux.Handle("GET /api/auth/account", handler.RequireIdentity(http.HandlerFunc(handler.handleAccount)))
	return handler, nil
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *HTTPHandler) RequireIdentity(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(SessionCookieName)
		if err != nil || cookie.Value == "" {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		account, err := h.sessions.Authenticate(r.Context(), cookie.Value, h.now().Unix())
		if errors.Is(err, ErrInvalidSession) {
			writeAuthError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		if err != nil {
			writeAuthError(w, http.StatusInternalServerError, "authentication unavailable")
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), identityContextKey{}, account)))
	})
}

func (h *HTTPHandler) RequireAdmin(next http.Handler) http.Handler {
	return h.RequireIdentity(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		account, ok := IdentityFromContext(r.Context())
		if !ok || account.Role != RoleAdmin {
			writeAuthError(w, http.StatusForbidden, "administrator access required")
			return
		}
		next.ServeHTTP(w, r)
	}))
}

func IdentityFromContext(ctx context.Context) (Account, bool) {
	account, ok := ctx.Value(identityContextKey{}).(Account)
	return account, ok
}

func (h *HTTPHandler) handleLogin(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readLoginWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "login temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if retryAfter, allowed := h.loginLimiter.allow(directPeerAddress(r.RemoteAddr), h.now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	account, err := h.authenticateWithinDeadline(ctx, request.Username, request.Password)
	if errors.Is(err, ErrInvalidCredentials) {
		writeAuthError(w, http.StatusUnauthorized, "invalid username or password")
		return
	}
	if errors.Is(err, ErrPasswordWorkOverloaded) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "login temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	credential, err := h.createSessionWithinDeadline(ctx, account, h.now().Unix())
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "login temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "login unavailable")
		return
	}
	h.setSessionCookie(w, r, credential.Token, 0)
	writeAuthJSON(w, http.StatusOK, publicAccount(account))
}

func (h *HTTPHandler) authenticateWithinDeadline(ctx context.Context, username, password string) (Account, error) {
	result := make(chan struct {
		account Account
		err     error
	}, 1)
	go func() {
		account, err := h.accounts.Authenticate(ctx, username, password)
		result <- struct {
			account Account
			err     error
		}{account: account, err: err}
	}()
	select {
	case authenticated := <-result:
		return authenticated.account, authenticated.err
	case <-ctx.Done():
		return Account{}, ctx.Err()
	}
}

func (h *HTTPHandler) createSessionWithinDeadline(ctx context.Context, account Account, now int64) (SessionCredential, error) {
	result := make(chan struct {
		credential SessionCredential
		err        error
	}, 1)
	go func() {
		credential, err := h.sessions.CreateAuthenticated(ctx, account, now)
		if err == nil && h.afterSessionCreate != nil {
			h.afterSessionCreate(credential)
		}
		result <- struct {
			credential SessionCredential
			err        error
		}{credential: credential, err: err}
	}()
	select {
	case created := <-result:
		return created.credential, created.err
	case <-ctx.Done():
		go h.revokeEventuallyCreatedSession(result)
		return SessionCredential{}, ctx.Err()
	}
}

func (h *HTTPHandler) revokeEventuallyCreatedSession(result <-chan struct {
	credential SessionCredential
	err        error
}) {
	created := <-result
	if created.err != nil || created.credential.Token == "" {
		return
	}
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		err := h.sessions.Logout(cleanupCtx, created.credential.Token)
		if err == nil {
			return
		}
		if cleanupCtx.Err() != nil {
			slog.Error("auth: failed to revoke session created after login timeout", "error", err)
			return
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-cleanupCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			slog.Error("auth: failed to revoke session created after login timeout", "error", cleanupCtx.Err())
			return
		case <-timer.C:
		}
	}
}

func (h *HTTPHandler) handleRegistrationPolicy(w http.ResponseWriter, _ *http.Request) {
	enabled, inviteRequired := false, false
	if h.registration != nil {
		enabled, inviteRequired = h.registration.Policy()
	}
	writeAuthJSON(w, http.StatusOK, registrationPolicyResponse{Enabled: enabled, InviteRequired: inviteRequired})
}

func (h *HTTPHandler) handleRegister(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	if h.registration == nil {
		writeAuthError(w, http.StatusNotFound, "registration unavailable")
		return
	}
	if retryAfter, allowed := h.loginLimiter.allow(directPeerAddress(r.RemoteAddr), h.now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many registration attempts")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readRegistrationWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "registration temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.registerWithinDeadline(ctx, request, h.now())
	switch {
	case errors.Is(err, ErrRegistrationUnavailable):
		writeAuthError(w, http.StatusForbidden, "registration unavailable")
		return
	case errors.Is(err, ErrUsernameUnavailable):
		writeAuthError(w, http.StatusConflict, "username unavailable")
		return
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword):
		writeAuthError(w, http.StatusBadRequest, "invalid username or password")
		return
	case errors.Is(err, ErrPasswordWorkOverloaded), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "registration temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "registration unavailable")
		return
	}
	credential, err := h.createSessionWithinDeadline(ctx, account, h.now().Unix())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "registration unavailable")
		return
	}
	h.setSessionCookie(w, r, credential.Token, 0)
	writeAuthJSON(w, http.StatusCreated, publicAccount(account))
}

func (h *HTTPHandler) registerWithinDeadline(ctx context.Context, request registrationRequest, now time.Time) (Account, error) {
	result := make(chan struct {
		account Account
		err     error
	}, 1)
	go func() {
		account, err := h.registerAccount(ctx, request.Username, request.Password, request.InviteCode, now)
		result <- struct {
			account Account
			err     error
		}{account: account, err: err}
	}()
	select {
	case completed := <-result:
		return completed.account, completed.err
	case <-ctx.Done():
		return Account{}, ctx.Err()
	}
}

func (h *HTTPHandler) handleLogout(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	if cookie, err := r.Cookie(SessionCookieName); err == nil {
		if err := h.sessions.Logout(r.Context(), cookie.Value); err != nil {
			writeAuthError(w, http.StatusInternalServerError, "logout unavailable")
			return
		}
	}
	h.setSessionCookie(w, r, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) handleAccount(w http.ResponseWriter, r *http.Request) {
	account, ok := IdentityFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusInternalServerError, "authentication unavailable")
		return
	}
	writeAuthJSON(w, http.StatusOK, publicAccount(account))
}

func (h *HTTPHandler) allowUnsafeRequest(w http.ResponseWriter, r *http.Request) bool {
	if _, _, ok := MatchRequestOrigin(h.allowedOrigin, r); ok {
		return true
	}
	writeAuthError(w, http.StatusForbidden, "origin not allowed")
	return false
}

func (l *loginRateLimiter) allow(peer string, now time.Time) (time.Duration, bool) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	attempt := l.attempts[peer]
	if attempt.start.IsZero() {
		if len(l.attempts) >= maxLoginRatePeers {
			for existingPeer, existingAttempt := range l.attempts {
				if now.Sub(existingAttempt.start) >= l.window {
					delete(l.attempts, existingPeer)
				}
			}
			if len(l.attempts) >= maxLoginRatePeers {
				return l.window, false
			}
		}
		l.attempts[peer] = loginAttemptWindow{count: 1, start: now}
		return 0, true
	}
	if now.Sub(attempt.start) >= l.window {
		l.attempts[peer] = loginAttemptWindow{count: 1, start: now}
		return 0, true
	}
	if attempt.count >= l.limit {
		return l.window - now.Sub(attempt.start), false
	}
	attempt.count++
	l.attempts[peer] = attempt
	return 0, true
}

func directPeerAddress(remoteAddr string) string {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil && host != "" {
		return host
	}
	return remoteAddr
}

func (h *HTTPHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int) {
	secure := h.secureCookie
	if h.allowedOrigin == "" {
		_, secure, _ = MatchRequestOrigin("", r)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   maxAge,
		Expires:  expiredCookieTime(maxAge),
	})
}

func expiredCookieTime(maxAge int) time.Time {
	if maxAge < 0 {
		return time.Unix(1, 0).UTC()
	}
	return time.Time{}
}

func MatchRequestOrigin(configuredOrigin string, r *http.Request) (string, bool, bool) {
	requestOrigin, secure, err := ParsePublicOrigin(r.Header.Get("Origin"))
	if err != nil || requestOrigin == "" {
		return "", false, false
	}
	if configuredOrigin != "" {
		return requestOrigin, secure, requestOrigin == configuredOrigin
	}
	parsed, err := url.Parse(requestOrigin)
	if err != nil || !sameOriginAuthority(parsed, r.Host) {
		return "", false, false
	}
	return requestOrigin, secure, true
}

func sameOriginAuthority(origin *url.URL, requestHost string) bool {
	requestAuthority, err := url.Parse("//" + requestHost)
	if err != nil || origin.Hostname() == "" || requestAuthority.Hostname() == "" || requestAuthority.User != nil {
		return false
	}
	originIP := net.ParseIP(origin.Hostname())
	requestIP := net.ParseIP(requestAuthority.Hostname())
	if originIP != nil || requestIP != nil {
		if originIP == nil || requestIP == nil || !originIP.Equal(requestIP) {
			return false
		}
	} else if !strings.EqualFold(origin.Hostname(), requestAuthority.Hostname()) {
		return false
	}
	originPort, err := validatedOriginPort(origin.Scheme, origin.Port())
	if err != nil {
		return false
	}
	requestPort, err := validatedOriginPort(origin.Scheme, requestAuthority.Port())
	return err == nil && originPort == requestPort
}

func validatedOriginPort(scheme, rawPort string) (int, error) {
	if rawPort == "" {
		if scheme == "https" {
			return 443, nil
		}
		return 80, nil
	}
	port, err := strconv.Atoi(rawPort)
	if err != nil || port < 1 || port > 65535 {
		return 0, errors.New("auth: origin port is invalid")
	}
	return port, nil
}

func ParsePublicOrigin(raw string) (string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || strings.HasSuffix(parsed.Host, ":") || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", false, errors.New("auth: PUBLIC_URL must be an HTTP(S) origin")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", false, errors.New("auth: PUBLIC_URL must not contain a path")
	}
	port, err := validatedOriginPort(parsed.Scheme, parsed.Port())
	if err != nil || parsed.Hostname() == "" {
		return "", false, errors.New("auth: PUBLIC_URL port is invalid")
	}
	host := strings.ToLower(parsed.Hostname())
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	defaultPort := parsed.Scheme == "http" && port == 80 || parsed.Scheme == "https" && port == 443
	if !defaultPort {
		portHost := strings.ToLower(parsed.Hostname())
		if ip := net.ParseIP(portHost); ip != nil {
			portHost = ip.String()
		}
		host = net.JoinHostPort(portHost, strconv.Itoa(port))
	}
	return parsed.Scheme + "://" + host, parsed.Scheme == "https", nil
}

type loginRequest struct {
	Username string
	Password string
}

var (
	errInvalidLoginRequest  = errors.New("auth: invalid login request")
	errLoginRequestTooLarge = errors.New("auth: login request is too large")
)

func readLoginWithinDeadline(ctx context.Context, r *http.Request) (loginRequest, error) {
	result := make(chan struct {
		request loginRequest
		err     error
	}, 1)
	go func() {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxLoginRequestSize+1))
		if err == nil && len(body) > maxLoginRequestSize {
			err = errLoginRequestTooLarge
		}
		var request loginRequest
		if err == nil {
			request, err = decodeLoginJSON(body)
		}
		result <- struct {
			request loginRequest
			err     error
		}{request: request, err: err}
	}()
	select {
	case decoded := <-result:
		return decoded.request, decoded.err
	case <-ctx.Done():
		_ = r.Body.Close()
		return loginRequest{}, ctx.Err()
	}
}

func readRegistrationWithinDeadline(ctx context.Context, r *http.Request) (registrationRequest, error) {
	result := make(chan struct {
		request registrationRequest
		err     error
	}, 1)
	go func() {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxLoginRequestSize+1))
		if err == nil && len(body) > maxLoginRequestSize {
			err = errLoginRequestTooLarge
		}
		var request registrationRequest
		if err == nil {
			request, err = decodeRegistrationJSON(body)
		}
		result <- struct {
			request registrationRequest
			err     error
		}{request, err}
	}()
	select {
	case decoded := <-result:
		return decoded.request, decoded.err
	case <-ctx.Done():
		_ = r.Body.Close()
		return registrationRequest{}, ctx.Err()
	}
}

func decodeRegistrationJSON(body []byte) (registrationRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return registrationRequest{}, errInvalidLoginRequest
	}
	var request registrationRequest
	seen := make(map[string]bool, 3)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return registrationRequest{}, errInvalidLoginRequest
		}
		seen[name] = true
		switch name {
		case "username":
			err = decoder.Decode(&request.Username)
		case "password":
			err = decoder.Decode(&request.Password)
		case "inviteCode":
			err = decoder.Decode(&request.InviteCode)
		default:
			return registrationRequest{}, errInvalidLoginRequest
		}
		if err != nil {
			return registrationRequest{}, errInvalidLoginRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["username"] || !seen["password"] {
		return registrationRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return registrationRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func decodeLoginJSON(body []byte) (loginRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return loginRequest{}, errInvalidLoginRequest
	}
	var request loginRequest
	seen := make(map[string]bool, 2)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return loginRequest{}, errInvalidLoginRequest
		}
		seen[name] = true
		switch name {
		case "username":
			err = decoder.Decode(&request.Username)
		case "password":
			err = decoder.Decode(&request.Password)
		default:
			return loginRequest{}, errInvalidLoginRequest
		}
		if err != nil {
			return loginRequest{}, errInvalidLoginRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["username"] || !seen["password"] {
		return loginRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return loginRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func publicAccount(account Account) accountResponse {
	return accountResponse{ID: string(account.ID), Username: account.Username, Role: account.Role}
}

func writeAuthJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	writeAuthJSON(w, status, map[string]string{"error": message})
}
