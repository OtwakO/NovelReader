package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"time"
)

func importControlHandler(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		handler(w, r.WithContext(ctx))
	}
}

func decodeImportRequest(w http.ResponseWriter, r *http.Request, value any, code, message string) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		writeErrorCode(w, http.StatusBadRequest, code, message)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeErrorCode(w, http.StatusBadRequest, code, "Expected one JSON object")
		return false
	}
	return true
}

func importPageLimit(r *http.Request) (int, error) {
	if r.URL.Query().Get("limit") == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 || limit > 100 {
		return 0, errors.New("limit must be between 1 and 100")
	}
	return limit, nil
}

// A context check before Read cannot interrupt an already blocked HTTP body read.
// Set a read deadline before closing; Close alone can wait on Read's HTTP/1 lock.
// Join the callback before resetting the deadline so it cannot affect keep-alive.
func interruptImportBody(ctx context.Context, w http.ResponseWriter, r *http.Request) (func() error, error) {
	controller := http.NewResponseController(w)
	deadline, _ := ctx.Deadline() // Admission.Begin always supplies a deadline.
	setDeadline := func(at time.Time) error {
		err := controller.SetReadDeadline(at)
		if errors.Is(err, http.ErrNotSupported) {
			return nil
		} // In-memory handlers have no socket.
		return err
	}
	if err := setDeadline(deadline); err != nil {
		return nil, err
	}
	done := make(chan struct{})
	var interruptErr error
	stop := context.AfterFunc(ctx, func() {
		interruptErr = errors.Join(setDeadline(time.Now()), r.Body.Close())
		close(done)
	})
	return func() error {
		if !stop() {
			<-done
		}
		return errors.Join(interruptErr, setDeadline(time.Time{}))
	}, nil
}

func inboxPage(w http.ResponseWriter, r *http.Request, code string) (string, int, bool) {
	limit, err := importPageLimit(r)
	after := r.URL.Query().Get("after")
	if err != nil || len(after) > 1024 {
		writeErrorCode(w, http.StatusBadRequest, code, "Invalid inbox page")
		return "", 0, false
	}
	return after, limit, true
}
