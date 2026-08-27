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
	SessionCookieName       = "novelreader_session"
	sessionCookieMaxAgeSecs = 30 * 24 * 60 * 60
	maxLoginRequestSize     = 4 * 1024
	maxLoginRatePeers       = 4096
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
	passwordLimiter    *loginRateLimiter
	resetLimiter       *loginRateLimiter
	now                func() time.Time
	afterSessionCreate func(SessionCredential)
	registration       *RegistrationService
	registerAccount    func(context.Context, string, string, string, time.Time) (Account, error)
	changePassword     func(context.Context, readerstore.UserID, string, string, int64) error
	setReaderEnabled   func(context.Context, readerstore.UserID, bool, int64) (Account, error)
	passwordResets     *PasswordResetService
	issuePasswordReset func(context.Context, readerstore.UserID, Account, int64) (PasswordResetCredential, error)
	completeReset      func(context.Context, string, string) error
	deletions          *DeletionService
	deleteReader       func(context.Context, readerstore.UserID, string, Account, int64) (DeletionJob, error)
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

type passwordChangeRequest struct {
	CurrentPassword string
	NewPassword     string
}

type readerStatusRequest struct {
	Enabled *bool
}

type passwordResetCompleteRequest struct {
	Token       string
	NewPassword string
}

type readerDeletionRequest struct {
	Username string
}

type accountResponse struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     Role   `json:"role"`
}

type passwordResetIssueResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expiresAt"`
}

type adminAccountResponse struct {
	ID        string        `json:"id"`
	Username  string        `json:"username"`
	Status    AccountStatus `json:"status"`
	CreatedAt int64         `json:"createdAt"`
	UpdatedAt int64         `json:"updatedAt"`
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
		passwordLimiter: &loginRateLimiter{
			attempts: make(map[string]loginAttemptWindow),
			limit:    config.LoginAttempts,
			window:   config.LoginWindow,
		},
		resetLimiter: &loginRateLimiter{
			attempts: make(map[string]loginAttemptWindow),
			limit:    config.LoginAttempts,
			window:   config.LoginWindow,
		},
		now: time.Now,
	}
	handler.changePassword = handler.accounts.ChangePassword
	handler.setReaderEnabled = handler.accounts.SetReaderEnabled
	handler.passwordResets = NewPasswordResetService(store)
	handler.passwordResets.now = func() int64 { return handler.now().Unix() }
	handler.issuePasswordReset = handler.passwordResets.Issue
	handler.completeReset = handler.passwordResets.Complete
	if config.Readers != nil {
		handler.registration = NewRegistrationService(store, config.Readers, config.RegistrationEnabled, config.RegistrationInviteCode)
		handler.registerAccount = handler.registration.Register
	}
	handler.mux.HandleFunc("POST /api/auth/login", handler.handleLogin)
	handler.mux.HandleFunc("POST /api/auth/logout", handler.handleLogout)
	handler.mux.HandleFunc("GET /api/auth/registration", handler.handleRegistrationPolicy)
	handler.mux.HandleFunc("POST /api/auth/register", handler.handleRegister)
	handler.mux.Handle("GET /api/auth/account", handler.RequireIdentity(http.HandlerFunc(handler.handleAccount)))
	handler.mux.Handle("POST /api/auth/password", handler.RequireIdentity(http.HandlerFunc(handler.handlePasswordChange)))
	handler.mux.HandleFunc("POST /api/auth/password-reset", handler.handlePasswordResetComplete)
	handler.mux.Handle("GET /api/auth/admin/readers", handler.RequireAdmin(http.HandlerFunc(handler.handleListReaders)))
	handler.mux.Handle("PUT /api/auth/admin/readers/{userID}/status", handler.RequireAdmin(http.HandlerFunc(handler.handleReaderStatus)))
	handler.mux.Handle("POST /api/auth/admin/readers/{userID}/password-reset", handler.RequireAdmin(http.HandlerFunc(handler.handlePasswordResetIssue)))
	handler.mux.Handle("DELETE /api/auth/admin/readers/{userID}", handler.RequireAdmin(http.HandlerFunc(handler.handleReaderDeletion)))
	return handler, nil
}

// ConfigureDeletionQuiescer completes deletion wiring after the API runtime manager exists.
func (h *HTTPHandler) ConfigureDeletionQuiescer(readers *readerstore.Manager, quiesce func(context.Context, readerstore.UserID) error) {
	h.deletions = NewDeletionService(h.accounts.store, readers, quiesce)
	h.deleteReader = h.deletions.Delete
}

func (h *HTTPHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	preventAuthResponseCaching(w)
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
	h.setSessionCookie(w, r, credential.Token, sessionCookieMaxAgeSecs)
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
	h.setSessionCookie(w, r, credential.Token, sessionCookieMaxAgeSecs)
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

func (h *HTTPHandler) handlePasswordChange(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	account, ok := IdentityFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusInternalServerError, "authentication unavailable")
		return
	}
	limitKey := string(account.ID) + "@" + directPeerAddress(r.RemoteAddr)
	if retryAfter, allowed := h.passwordLimiter.allow(limitKey, h.now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many password change attempts")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readPasswordChangeWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "password change temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err = h.changePasswordWithinDeadline(ctx, account.ID, request, h.now().Unix())
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		writeAuthError(w, http.StatusForbidden, "invalid current password")
		return
	case errors.Is(err, ErrInvalidPassword):
		writeAuthError(w, http.StatusBadRequest, "invalid new password")
		return
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		h.setSessionCookie(w, r, "", -1)
		writeAuthError(w, http.StatusUnauthorized, "password change outcome requires sign in")
		return
	case errors.Is(err, ErrPasswordWorkOverloaded):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "password change temporarily unavailable")
		return
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "password change unavailable")
		return
	}
	h.setSessionCookie(w, r, "", -1)
	w.WriteHeader(http.StatusNoContent)
}

func (h *HTTPHandler) changePasswordWithinDeadline(ctx context.Context, userID readerstore.UserID, request passwordChangeRequest, now int64) error {
	result := make(chan error, 1)
	go func() {
		result <- h.changePassword(ctx, userID, request.CurrentPassword, request.NewPassword, now)
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *HTTPHandler) handleReaderDeletion(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	if h.deleteReader == nil {
		writeAuthError(w, http.StatusServiceUnavailable, "account deletion unavailable")
		return
	}
	userID, err := readerstore.ParseUserID(r.PathValue("userID"))
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid reader account")
		return
	}
	issuer, ok := IdentityFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readReaderDeletionWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		writeAuthError(w, http.StatusServiceUnavailable, "account deletion temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	job, err := h.deleteReader(ctx, userID, request.Username, issuer, h.now().Unix())
	switch {
	case errors.Is(err, ErrProtectedAccount):
		writeAuthError(w, http.StatusForbidden, "protected account")
	case errors.Is(err, ErrUsernameConfirmation):
		writeAuthError(w, http.StatusBadRequest, "username confirmation does not match")
	case errors.Is(err, ErrAccountNotFound):
		writeAuthError(w, http.StatusNotFound, "reader account not found")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "account deletion continues; retry to check completion")
	case err != nil:
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "account deletion failed; retry to continue")
	default:
		writeAuthJSON(w, http.StatusOK, map[string]any{"status": job.Status})
	}
}

func (h *HTTPHandler) handlePasswordResetIssue(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	userID, err := readerstore.ParseUserID(r.PathValue("userID"))
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid reader account")
		return
	}
	issuer, ok := IdentityFromContext(r.Context())
	if !ok {
		writeAuthError(w, http.StatusUnauthorized, "authentication required")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	credential, err := h.issuePasswordReset(ctx, userID, issuer, h.now().Unix())
	switch {
	case errors.Is(err, ErrProtectedAccount):
		writeAuthError(w, http.StatusForbidden, "protected account")
	case errors.Is(err, ErrAccountNotFound):
		writeAuthError(w, http.StatusNotFound, "reader account not found")
	case errors.Is(err, ErrAccountNotActive):
		writeAuthError(w, http.StatusConflict, "reader account unavailable")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "password reset temporarily unavailable")
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "password reset unavailable")
	default:
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Pragma", "no-cache")
		writeAuthJSON(w, http.StatusCreated, passwordResetIssueResponse{Token: credential.Token, ExpiresAt: credential.ExpiresAt})
	}
}

func (h *HTTPHandler) handlePasswordResetComplete(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	if retryAfter, allowed := h.resetLimiter.allow(directPeerAddress(r.RemoteAddr), h.now()); !allowed {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retryAfter.Seconds()))))
		writeAuthError(w, http.StatusTooManyRequests, "too many password reset attempts")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readPasswordResetCompleteWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "password reset temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	err = h.completePasswordResetWithinDeadline(ctx, request)
	switch {
	case errors.Is(err, ErrInvalidPasswordReset):
		writeAuthError(w, http.StatusBadRequest, "invalid or expired password reset")
	case errors.Is(err, ErrInvalidPassword):
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		writeAuthError(w, http.StatusConflict, "password reset outcome is uncertain; try signing in with the new password")
	case errors.Is(err, ErrPasswordWorkOverloaded):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "password reset temporarily unavailable")
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "password reset unavailable")
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

func (h *HTTPHandler) completePasswordResetWithinDeadline(ctx context.Context, request passwordResetCompleteRequest) error {
	result := make(chan error, 1)
	go func() {
		result <- h.completeReset(ctx, request.Token, request.NewPassword)
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *HTTPHandler) handleListReaders(w http.ResponseWriter, r *http.Request) {
	accounts, err := h.accounts.ListReaderAccounts(r.Context())
	if err != nil {
		writeAuthError(w, http.StatusInternalServerError, "account administration unavailable")
		return
	}
	response := make([]adminAccountResponse, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, adminPublicAccount(account))
	}
	writeAuthJSON(w, http.StatusOK, map[string]any{"accounts": response})
}

func (h *HTTPHandler) handleReaderStatus(w http.ResponseWriter, r *http.Request) {
	if !h.allowUnsafeRequest(w, r) {
		return
	}
	userID, err := readerstore.ParseUserID(r.PathValue("userID"))
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid reader account")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), h.loginTimeout)
	defer cancel()
	request, err := readReaderStatusWithinDeadline(ctx, r)
	if errors.Is(err, errLoginRequestTooLarge) {
		writeAuthError(w, http.StatusRequestEntityTooLarge, "request body is too large")
		return
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "account administration temporarily unavailable")
		return
	}
	if err != nil {
		writeAuthError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	account, err := h.setReaderEnabled(ctx, userID, *request.Enabled, h.now().Unix())
	switch {
	case errors.Is(err, ErrProtectedAccount):
		writeAuthError(w, http.StatusForbidden, "protected account")
	case errors.Is(err, ErrAccountNotFound):
		writeAuthError(w, http.StatusNotFound, "reader account not found")
	case errors.Is(err, ErrInvalidStatusTransition):
		writeAuthError(w, http.StatusConflict, "reader account status changed")
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		w.Header().Set("Retry-After", "1")
		writeAuthError(w, http.StatusServiceUnavailable, "account administration temporarily unavailable")
	case err != nil:
		writeAuthError(w, http.StatusInternalServerError, "account administration unavailable")
	default:
		writeAuthJSON(w, http.StatusOK, adminPublicAccount(account))
	}
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
	setBrowserSessionCookie(w, r, value, maxAge, h.secureCookie, h.allowedOrigin)
}

func preventAuthResponseCaching(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
}

func setBrowserSessionCookie(w http.ResponseWriter, r *http.Request, value string, maxAge int, secureCookie bool, allowedOrigin string) {
	secure := secureCookie
	if allowedOrigin == "" {
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

func readPasswordChangeWithinDeadline(ctx context.Context, r *http.Request) (passwordChangeRequest, error) {
	body, err := readAuthBodyWithinDeadline(ctx, r)
	if err != nil {
		return passwordChangeRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return passwordChangeRequest{}, errInvalidLoginRequest
	}
	var request passwordChangeRequest
	seen := make(map[string]bool, 2)
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return passwordChangeRequest{}, errInvalidLoginRequest
		}
		seen[name] = true
		switch name {
		case "currentPassword":
			err = decoder.Decode(&request.CurrentPassword)
		case "newPassword":
			err = decoder.Decode(&request.NewPassword)
		default:
			return passwordChangeRequest{}, errInvalidLoginRequest
		}
		if err != nil {
			return passwordChangeRequest{}, errInvalidLoginRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen["currentPassword"] || !seen["newPassword"] {
		return passwordChangeRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return passwordChangeRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func readReaderDeletionWithinDeadline(ctx context.Context, r *http.Request) (readerDeletionRequest, error) {
	body, err := readAuthBodyWithinDeadline(ctx, r)
	if err != nil {
		return readerDeletionRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return readerDeletionRequest{}, errInvalidLoginRequest
	}
	var request readerDeletionRequest
	seen := false
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen || name != "username" {
			return readerDeletionRequest{}, errInvalidLoginRequest
		}
		seen = true
		if err := decoder.Decode(&request.Username); err != nil {
			return readerDeletionRequest{}, errInvalidLoginRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || !seen || request.Username == "" {
		return readerDeletionRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return readerDeletionRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func readPasswordResetCompleteWithinDeadline(ctx context.Context, r *http.Request) (passwordResetCompleteRequest, error) {
	body, err := readAuthBodyWithinDeadline(ctx, r)
	if err != nil {
		return passwordResetCompleteRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return passwordResetCompleteRequest{}, errInvalidLoginRequest
	}
	var request passwordResetCompleteRequest
	seen := map[string]bool{}
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen[name] {
			return passwordResetCompleteRequest{}, errInvalidLoginRequest
		}
		seen[name] = true
		switch name {
		case "token":
			if err := decoder.Decode(&request.Token); err != nil {
				return passwordResetCompleteRequest{}, errInvalidLoginRequest
			}
		case "newPassword":
			if err := decoder.Decode(&request.NewPassword); err != nil {
				return passwordResetCompleteRequest{}, errInvalidLoginRequest
			}
		default:
			return passwordResetCompleteRequest{}, errInvalidLoginRequest
		}
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || request.Token == "" || request.NewPassword == "" {
		return passwordResetCompleteRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return passwordResetCompleteRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func readReaderStatusWithinDeadline(ctx context.Context, r *http.Request) (readerStatusRequest, error) {
	body, err := readAuthBodyWithinDeadline(ctx, r)
	if err != nil {
		return readerStatusRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return readerStatusRequest{}, errInvalidLoginRequest
	}
	var request readerStatusRequest
	seen := false
	for decoder.More() {
		field, err := decoder.Token()
		name, ok := field.(string)
		if err != nil || !ok || seen || name != "enabled" {
			return readerStatusRequest{}, errInvalidLoginRequest
		}
		seen = true
		var enabled bool
		if err := decoder.Decode(&enabled); err != nil {
			return readerStatusRequest{}, errInvalidLoginRequest
		}
		request.Enabled = &enabled
	}
	closing, err := decoder.Token()
	if err != nil || closing != json.Delim('}') || request.Enabled == nil {
		return readerStatusRequest{}, errInvalidLoginRequest
	}
	if _, err := decoder.Token(); err != io.EOF {
		return readerStatusRequest{}, errInvalidLoginRequest
	}
	return request, nil
}

func readAuthBodyWithinDeadline(ctx context.Context, r *http.Request) ([]byte, error) {
	result := make(chan struct {
		body []byte
		err  error
	}, 1)
	go func() {
		defer r.Body.Close()
		body, err := io.ReadAll(io.LimitReader(r.Body, maxLoginRequestSize+1))
		if err == nil && len(body) > maxLoginRequestSize {
			err = errLoginRequestTooLarge
		}
		result <- struct {
			body []byte
			err  error
		}{body, err}
	}()
	select {
	case read := <-result:
		return read.body, read.err
	case <-ctx.Done():
		_ = r.Body.Close()
		return nil, ctx.Err()
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

func adminPublicAccount(account Account) adminAccountResponse {
	return adminAccountResponse{
		ID: string(account.ID), Username: account.Username, Status: account.Status,
		CreatedAt: account.CreatedAt, UpdatedAt: account.UpdatedAt,
	}
}

func writeAuthJSON(w http.ResponseWriter, status int, value interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAuthError(w http.ResponseWriter, status int, message string) {
	writeAuthJSON(w, status, map[string]string{"error": message})
}
