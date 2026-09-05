package auth

import (
	"context"
	"log/slog"
	"time"
)

const (
	lateSessionCleanupTimeout = 5 * time.Second
	lateSessionRetryInterval  = 10 * time.Millisecond
)

type sessionCreationResult struct {
	credential SessionCredential
	err        error
}

// createAuthenticatedSessionWithinDeadline keeps HTTP auth flows responsive when
// persistent session creation is blocked. A session that commits after the caller's
// deadline is revoked asynchronously so a timed-out response cannot leave a valid,
// undisclosed browser credential behind.
func createAuthenticatedSessionWithinDeadline(
	ctx context.Context,
	sessions *SessionService,
	account Account,
	now int64,
	afterCreate func(SessionCredential),
	flow string,
) (SessionCredential, error) {
	result := make(chan sessionCreationResult, 1)
	go func() {
		credential, err := sessions.CreateAuthenticated(ctx, account, now)
		if err == nil && afterCreate != nil {
			afterCreate(credential)
		}
		result <- sessionCreationResult{credential: credential, err: err}
	}()

	select {
	case created := <-result:
		return created.credential, created.err
	case <-ctx.Done():
		go revokeLateSession(sessions, result, flow)
		return SessionCredential{}, ctx.Err()
	}
}

func revokeLateSession(sessions *SessionService, result <-chan sessionCreationResult, flow string) {
	created := <-result
	if created.err != nil || created.credential.Token == "" {
		return
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), lateSessionCleanupTimeout)
	defer cancel()
	for {
		err := sessions.Logout(cleanupCtx, created.credential.Token)
		if err == nil {
			return
		}
		if cleanupCtx.Err() != nil {
			slog.Error("auth: failed to revoke session created after response deadline", "flow", flow, "error", err)
			return
		}

		timer := time.NewTimer(lateSessionRetryInterval)
		select {
		case <-cleanupCtx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			slog.Error("auth: failed to revoke session created after response deadline", "flow", flow, "error", cleanupCtx.Err())
			return
		case <-timer.C:
		}
	}
}
