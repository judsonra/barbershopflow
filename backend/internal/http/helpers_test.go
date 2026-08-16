package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/barberflow/backend/internal/auth"
	"github.com/example/barberflow/backend/internal/domain"
	"github.com/example/barberflow/backend/internal/notify"
)

// errUnmapped stands in for an error handleError has no specific mapping
// for, exercising its default branch (500/internal_error).
var errUnmapped = errors.New("unmapped store error")

// fakeNotifier records every SendPassword call instead of reaching out to
// Zenvia, so handler tests can assert a password/notification was (or
// wasn't) sent without any network access.
type fakeNotifier struct {
	sent []notifierCall
	err  error
}

type notifierCall struct {
	Phone, Password, Channel string
}

func (f *fakeNotifier) SendPassword(_ context.Context, phone, password, channel string) error {
	if f.err != nil {
		return f.err
	}
	f.sent = append(f.sent, notifierCall{Phone: phone, Password: password, Channel: channel})
	return nil
}

var _ notify.Notifier = (*fakeNotifier)(nil)

const testTokenSecret = "test-secret"

// newTestServer builds the real handler chain (routing + middleware) from
// New, backed by store, with a tokenizer tests can use to mint auth headers.
func newTestServer(store *fakeStore) (http.Handler, *auth.Tokenizer, *fakeNotifier) {
	tok := auth.NewTokenizer(testTokenSecret, 15*time.Minute, 168*time.Hour)
	notifier := &fakeNotifier{}
	handler := New(store, Config{
		Origins:     "http://localhost",
		Tokenizer:   tok,
		Notifier:    notifier,
		FrontendURL: "http://localhost:5173",
	})
	return handler, tok, notifier
}

// authHeader mints a valid access token for user and returns it as a ready
// to use Authorization header value.
func authHeader(t *testing.T, tok *auth.Tokenizer, user domain.User) string {
	t.Helper()
	token, err := tok.GenerateAccessToken(user)
	if err != nil {
		t.Fatalf("generate access token: %v", err)
	}
	return "Bearer " + token
}

// doRequest performs one request against handler and returns the recorded
// response. body, when non-nil, is JSON-encoded; authorization may be "" to
// omit the header entirely.
func doRequest(t *testing.T, handler http.Handler, method, path, authorization string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(encoded)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// doRawRequest is like doRequest but sends rawBody verbatim, for exercising
// decode's malformed-JSON / unknown-field rejection paths that a
// marshal-from-struct call could never produce.
func doRawRequest(t *testing.T, handler http.Handler, method, path, authorization, rawBody string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(rawBody))
	req.Header.Set("Content-Type", "application/json")
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// decodeBody unmarshals rec's JSON body into dst, failing the test on error.
func decodeBody(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), dst); err != nil {
		t.Fatalf("decode response body %q: %v", rec.Body.String(), err)
	}
}

// errorCode extracts the "error.code" field from a handleError response body.
func errorCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	decodeBody(t, rec, &body)
	return body.Error.Code
}
