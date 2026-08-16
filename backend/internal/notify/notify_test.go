package notify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_NoTokenReturnsLogNotifier(t *testing.T) {
	notifier := New("", "from-sms", "from-whatsapp")
	if _, ok := notifier.(logNotifier); !ok {
		t.Fatalf("expected a logNotifier when apiToken is empty, got %T", notifier)
	}
}

func TestNew_WithTokenReturnsZenviaNotifier(t *testing.T) {
	notifier := New("token", "from-sms", "from-whatsapp")
	if _, ok := notifier.(*zenviaNotifier); !ok {
		t.Fatalf("expected a *zenviaNotifier when apiToken is set, got %T", notifier)
	}
}

func TestLogNotifier_NeverErrors(t *testing.T) {
	if err := (logNotifier{}).SendPassword(context.Background(), "+5511999990000", "123456", ChannelSMS); err != nil {
		t.Fatalf("expected logNotifier to never error, got %v", err)
	}
}

func newTestZenviaNotifier(baseURL string) *zenviaNotifier {
	return &zenviaNotifier{
		apiToken: "test-token", fromSMS: "SMS_FROM", fromWhatsApp: "WA_FROM",
		client: http.DefaultClient, baseURL: baseURL,
	}
}

func TestZenviaNotifier_SendPasswordHappyPath(t *testing.T) {
	var gotPath, gotToken, gotFrom string
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotToken = r.Header.Get("X-API-TOKEN")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		gotFrom, _ = gotBody["from"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := newTestZenviaNotifier(server.URL)
	if err := notifier.SendPassword(context.Background(), "+5511999990000", "654321", ChannelWhatsApp); err != nil {
		t.Fatalf("SendPassword: %v", err)
	}
	if gotPath != "/v2/channels/whatsapp/messages" {
		t.Fatalf("unexpected request path %q", gotPath)
	}
	if gotToken != "test-token" {
		t.Fatalf("expected X-API-TOKEN header, got %q", gotToken)
	}
	if gotFrom != "WA_FROM" {
		t.Fatalf("expected the whatsapp from number, got %q", gotFrom)
	}
	if to, _ := gotBody["to"].(string); to != "+5511999990000" {
		t.Fatalf("expected the target phone in the body, got %v", gotBody["to"])
	}
}

func TestZenviaNotifier_SendPasswordUsesSMSFrom(t *testing.T) {
	var gotFrom string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotFrom, _ = body["from"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	notifier := newTestZenviaNotifier(server.URL)
	if err := notifier.SendPassword(context.Background(), "+5511999990000", "654321", ChannelSMS); err != nil {
		t.Fatalf("SendPassword: %v", err)
	}
	if gotFrom != "SMS_FROM" {
		t.Fatalf("expected the sms from number, got %q", gotFrom)
	}
}

func TestZenviaNotifier_NonSuccessStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	notifier := newTestZenviaNotifier(server.URL)
	if err := notifier.SendPassword(context.Background(), "+5511999990000", "654321", ChannelSMS); err == nil {
		t.Fatal("expected an error for a non-2xx Zenvia response")
	}
}

func TestZenviaNotifier_NetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	server.Close() // closed before use, so any request fails to connect

	notifier := newTestZenviaNotifier(server.URL)
	if err := notifier.SendPassword(context.Background(), "+5511999990000", "654321", ChannelSMS); err == nil {
		t.Fatal("expected an error when the Zenvia endpoint is unreachable")
	}
}
