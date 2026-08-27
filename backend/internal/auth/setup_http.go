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
	maxSetupRequestSize = 8 * 1024
	defaultSetupTimeout = 10 * time.Second
)

type SetupHTTPConfig struct {
	PublicURL      string
	BootstrapToken string
	Timeout        time.Duration
}

type SetupHTTPHandler struct {
	setup              *SetupService
	sessions           *SessionService
	mux                *http.ServeMux
	allowedOrigin      string
	secureCookie       bool
	timeout            time.Duration
	now                func() time.Time
	setupLimiter       *loginRateLimiter
	afterActivation    func(Account)
	afterSessionCreate func(SessionCredential)
}

type setupStatusResponse struct {
	Status    string `json:"status"`
	Available bool   `json:"available"`
}

type setupRequest struct {
	Token    string
	Username string
	Password string
}

func NewSetupHTTPHandler(store *Store, readers *readerstore.Manager, config SetupHTTPConfig) (*SetupHTTPHandler, error) {
	origin, secure, err := ParsePublicOrigin(config.PublicURL)
	if err != nil {
		return nil, err
	}
	if config.Timeout <= 0 {
		config.Timeout = defaultSetupTimeout
	}
	handler := &SetupHTTPHandler{
		setup:         NewSetupService(store, readers, config.BootstrapToken),
		sessions:      NewSessionService(store),
		mux:           http.NewServeMux(),
		allowedOrigin: origin,
		secureCookie:  secure,
		timeout:       config.Timeout,
		now:           time.Now,
		setupLimiter: &loginRateLimiter{
			attempts: make(map[string]loginAttemptWindow),
			limit:    10,
			window:   time.Minute,
		},
	}
	handler.mux.HandleFunc("GET /api/setup/status", handler.handleStatus)
	handler.mux.HandleFunc("POST /api/setup", handler.handleSetup)
	return handler, nil
}

func (h *SetupHTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	preventAuthResponseCaching(w)
	h.mux.ServeHTTP(w, r)
}

func (h *SetupHTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	status, err := h.setup.SetupStatus(r.Context())
	if err != nil {
		writeAuthError(w, http.StatusServiceUnavailable, "setup unavailable")
		return
	}
	writeAuthJSON(w, http.StatusOK, setupStatusResponse{
		Status:    status,
		Available: status != "closed" && h.setup.bootstrapToken != "",
	})
}

func (h *SetupHTTPHandler) handleSetup(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := MatchRequestOrigin(h.allowedOrigin, r); !ok {
		writeAuthError(w, http.StatusForbidden, "origin not allowed")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.timeout)
	defer cancel()
	request, err := readSetupWithinDeadline(ctx, r)
	switch {
	case errors.Is(err, errSetupRequestTooLarge):
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled), errors.Is(err, ErrPasswordWorkOverloaded):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "setup temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if !validBootstrapToken(h.setup.bootstrapToken, request.Token) {
		writeAuthError(w, http.StatusUnauthorized, "invalid bootstrap token")
		return
	}
	now := h.now()
	if retryAfter, allowed := h.setupLimiter.allow(directPeerAddress(r.RemoteAddr), now); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many setup attempts")
		return
	}
	account, err := h.performSetupWithinDeadline(ctx, request, now)
	switch {
	case errors.Is(err, ErrSetupUnavailable):
		writeAuthError(w, http.StatusUnauthorized, "invalid bootstrap token")
		return
	case errors.Is(err, ErrInvalidCredentials):
		writeAuthError(w, http.StatusConflict, "setup is already complete")
		return
	case errors.Is(err, ErrInvalidUsername), errors.Is(err, ErrInvalidPassword):
		writeAuthError(w, http.StatusBadRequest, "invalid username or password")
		return
	case errors.Is(err, ErrPasswordWorkOverloaded), errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "setup temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "setup unavailable")
		return
	}

	sessionCtx, sessionCancel := context.WithTimeout(r.Context(), h.timeout)
	defer sessionCancel()
	credential, err := h.createSessionWithinDeadline(sessionCtx, account, now.Unix())
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "setup temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "setup unavailable")
		return
	}
	h.setSessionCookie(w, r, credential.Token)
	writeAuthJSON(w, http.StatusCreated, publicAccount(account))
}

func (h *SetupHTTPHandler) performSetupWithinDeadline(ctx context.Context, request setupRequest, now time.Time) (Account, error) {
	result := make(chan struct {
		account Account
		err     error
	}, 1)
	go func() {
		account, err := h.setup.CreateInitialAdministrator(ctx, request.Token, request.Username, request.Password, now)
		if errors.Is(err, ErrSetupInProgress) {
			account, err = h.setup.RecoverInitialAdministrator(ctx, request.Token, now)
		}
		if errors.Is(err, ErrSetupClosed) {
			account, err = h.setup.AuthenticateInitialAdministrator(ctx, request.Username, request.Password)
		}
		if err == nil && h.afterActivation != nil {
			h.afterActivation(account)
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

func (h *SetupHTTPHandler) createSessionWithinDeadline(ctx context.Context, account Account, now int64) (SessionCredential, error) {
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

func (h *SetupHTTPHandler) revokeEventuallyCreatedSession(result <-chan struct {
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
			slog.Error("auth: failed to revoke session created after setup timeout", "error", err)
			return
		}
		timer := time.NewTimer(10 * time.Millisecond)
		select {
		case <-cleanupCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			slog.Error("auth: failed to revoke session created after setup timeout", "error", cleanupCtx.Err())
			return
		case <-timer.C:
		}
	}
}

func (h *SetupHTTPHandler) setSessionCookie(w http.ResponseWriter, r *http.Request, token string) {
	setBrowserSessionCookie(w, r, token, sessionCookieMaxAgeSecs, h.secureCookie, h.allowedOrigin)
}

var (
	errInvalidSetupRequest  = errors.New("auth: invalid setup request")
	errSetupRequestTooLarge = errors.New("auth: setup request is too large")
)

func readSetupWithinDeadline(ctx context.Context, r *http.Request) (setupRequest, error) {
	result := make(chan struct {
		request setupRequest
		err     error
	}, 1)
	go func() {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxSetupRequestSize+1))
		if err == nil && len(body) > maxSetupRequestSize {
			err = errSetupRequestTooLarge
		}
		var request setupRequest
		if err == nil {
			request, err = decodeSetupJSON(body)
		}
		result <- struct {
			request setupRequest
			err     error
		}{request: request, err: err}
	}()
	select {
	case decoded := <-result:
		return decoded.request, decoded.err
	case <-ctx.Done():
		_ = r.Body.Close()
		return setupRequest{}, ctx.Err()
	}
}

func decodeSetupJSON(body []byte) (setupRequest, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return setupRequest{}, errInvalidSetupRequest
	}
	var request setupRequest
	seen := make(map[string]bool, 3)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return setupRequest{}, errInvalidSetupRequest
		}
		seen[name] = true
		switch name {
		case "token":
			err = decoder.Decode(&request.Token)
		case "username":
			err = decoder.Decode(&request.Username)
		case "password":
			err = decoder.Decode(&request.Password)
		default:
			return setupRequest{}, errInvalidSetupRequest
		}
		if err != nil {
			return setupRequest{}, errInvalidSetupRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["token"] || !seen["username"] || !seen["password"] {
		return setupRequest{}, errInvalidSetupRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return setupRequest{}, errInvalidSetupRequest
	}
	return request, nil
}
