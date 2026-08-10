package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/otwako/novelreader/internal/readerstore"
)

const (
	maxRecoveryRequestSize = 8 * 1024
	defaultRecoveryTimeout = 10 * time.Second
)

type RecoveryHTTPConfig struct {
	PublicURL     string
	RecoveryToken string
	Timeout       time.Duration
}

type RecoveryHTTPHandler struct {
	recovery           *RecoveryService
	sessions           *SessionService
	mux                *http.ServeMux
	allowedOrigin      string
	secureCookie       bool
	timeout            time.Duration
	now                func() time.Time
	recoveryLimiter    *loginRateLimiter
	afterRecovery      func(Account)
	afterSessionCreate func(SessionCredential)
}

type recoveryStatusResponse struct {
	Available bool `json:"available"`
}

type recoveryRequest struct {
	Token    string
	Action   RecoveryAction
	Username string
	Password string
}

func NewRecoveryHTTPHandler(store *Store, readers *readerstore.Manager, config RecoveryHTTPConfig) (*RecoveryHTTPHandler, error) {
	origin, secure, err := ParsePublicOrigin(config.PublicURL)
	if err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultRecoveryTimeout
	}
	handler := &RecoveryHTTPHandler{
		recovery:      NewRecoveryService(store, readers, config.RecoveryToken),
		sessions:      NewSessionService(store),
		mux:           http.NewServeMux(),
		allowedOrigin: origin,
		secureCookie:  secure,
		timeout:       config.Timeout,
		now:           time.Now,
		recoveryLimiter: &loginRateLimiter{
			attempts: make(map[string]loginAttemptWindow),
			limit:    10,
			window:   time.Minute,
		},
	}
	handler.mux.HandleFunc("GET /api/recovery/status", handler.handleStatus)
	handler.mux.HandleFunc("POST /api/recovery", handler.handleRecovery)
	return handler, nil
}

func (h *RecoveryHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *RecoveryHTTPHandler) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeAuthJSON(w, http.StatusOK, recoveryStatusResponse{Available: h.recovery.recoveryToken != ""})
}

func (h *RecoveryHTTPHandler) handleRecovery(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := MatchRequestOrigin(h.allowedOrigin, r); !ok {
		writeAuthError(w, http.StatusForbidden, "origin not allowed")
		return
	}
	now := h.now()
	if retryAfter, allowed := h.recoveryLimiter.allow(directPeerAddress(r.RemoteAddr), now); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many recovery attempts")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	request, err := readRecoveryWithinDeadline(ctx, r)
	switch {
	case errors.Is(err, errRecoveryRequestTooLarge):
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "recovery temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.performRecoveryWithinDeadline(ctx, request, now)
	switch {
	case errors.Is(err, ErrRecoveryUnavailable):
		writeAuthError(w, http.StatusUnauthorized, "invalid recovery token")
		return
	case errors.Is(err, ErrInvalidRecoveryAction), errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword):
		writeAuthError(w, http.StatusBadRequest, "invalid recovery request")
		return
	case errors.Is(err, ErrRecoveryTarget), errors.Is(err, ErrRecoveryInProgress), errors.Is(err, ErrUsernameUnavailable):
		writeAuthError(w, http.StatusConflict, "recovery could not be completed")
		return
	case errors.Is(err, ErrPasswordWorkOverloaded), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "recovery temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "recovery unavailable")
		return
	}

	sessionCtx, sessionCancel := context.WithTimeout(r.Context(), h.timeout)
	defer sessionCancel()
	credential, err := h.createSessionWithinDeadline(sessionCtx, account, now.Unix())
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "recovery temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "recovery unavailable")
		return
	}
	h.setSessionCookie(w, r, credential.Token)
	writeAuthJSON(w, http.StatusOK, publicAccount(account))
}

func (h *RecoveryHTTPHandler) performRecoveryWithinDeadline(ctx context.Context, request recoveryRequest, now time.Time) (Account, error) {
	result := make(chan struct {
		account Account
		err     error
	}, 1)
	go func() {
		account, err := h.recovery.Recover(ctx, request.Token, request.Action, request.Username, request.Password, now)
		if err == nil && h.afterRecovery != nil {
			h.afterRecovery(account)
		}
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

func (h *RecoveryHTTPHandler) createSessionWithinDeadline(ctx context.Context, account Account, now int64) (SessionCredential, error) {
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

func (h *RecoveryHTTPHandler) revokeEventuallyCreatedSession(result <-chan struct {
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
			slog.Error("auth: failed to revoke session created after recovery timeout", "error", err)
			return
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-cleanupCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			slog.Error("auth: failed to revoke session created after recovery timeout", "error", cleanupCtx.Err())
			return
		case <-timer.C:
		}
	}
}

func (h *RecoveryHTTPHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	secure := h.secureCookie
	if h.allowedOrigin == "" {
		_, secure, _ = MatchRequestOrigin("", r)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     SessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
	})
}

var (
	errInvalidRecoveryRequest  = errors.New("auth: invalid recovery request")
	errRecoveryRequestTooLarge = errors.New("auth: recovery request is too large")
)

func readRecoveryWithinDeadline(ctx context.Context, r *http.Request) (recoveryRequest, error) {
	result := make(chan struct {
		request recoveryRequest
		err     error
	}, 1)
	go func() {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxRecoveryRequestSize+1))
		if err == nil && len(body) > maxRecoveryRequestSize {
			err = errRecoveryRequestTooLarge
		}
		var request recoveryRequest
		if err == nil {
			request, err = decodeRecoveryJSON(body)
		}
		result <- struct {
			request recoveryRequest
			err     error
		}{request: request, err: err}
	}()
	select {
	case decoded := <-result:
		return decoded.request, decoded.err
	case <-ctx.Done():
		return recoveryRequest{}, ctx.Err()
	}
}

func decodeRecoveryJSON(body []byte) (recoveryRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return recoveryRequest{}, errInvalidRecoveryRequest
	}
	var request recoveryRequest
	seen := make(map[string]bool, 4)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return recoveryRequest{}, errInvalidRecoveryRequest
		}
		seen[name] = true
		switch name {
		case "token":
			err = decoder.Decode(&request.Token)
		case "action":
			err = decoder.Decode(&request.Action)
		case "username":
			err = decoder.Decode(&request.Username)
		case "password":
			err = decoder.Decode(&request.Password)
		default:
			return recoveryRequest{}, errInvalidRecoveryRequest
		}
		if err != nil {
			return recoveryRequest{}, errInvalidRecoveryRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["token"] || !seen["action"] || !seen["username"] || !seen["password"] {
		return recoveryRequest{}, errInvalidRecoveryRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return recoveryRequest{}, errInvalidRecoveryRequest
	}
	if request.Token == "" || request.Username == "" || request.Password == "" || (request.Action != RecoveryResetExisting && request.Action != RecoveryCreateReplacement) {
		return recoveryRequest{}, errInvalidRecoveryRequest
	}
	return request, nil
}
