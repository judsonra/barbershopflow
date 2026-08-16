// Package notify sends the temporary/reset password to clients who log in
// with phone + password instead of e-mail. Falls back to logging the
// message when no provider is configured, so the flow is testable in dev
// without a Zenvia account.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

const (
	ChannelSMS      = "sms"
	ChannelWhatsApp = "whatsapp"
)

type Notifier interface {
	// SendPassword delivers a login password to phone over the given
	// channel ("sms" or "whatsapp").
	SendPassword(ctx context.Context, phone, password, channel string) error
}

const zenviaBaseURL = "https://api.zenvia.com"

func New(apiToken, fromSMS, fromWhatsApp string) Notifier {
	if apiToken == "" {
		return logNotifier{}
	}
	return &zenviaNotifier{
		apiToken:     apiToken,
		fromSMS:      fromSMS,
		fromWhatsApp: fromWhatsApp,
		client:       &http.Client{Timeout: 10 * time.Second},
		baseURL:      zenviaBaseURL,
	}
}

type logNotifier struct{}

func (logNotifier) SendPassword(_ context.Context, phone, password, channel string) error {
	log.Printf("[notify:stub] %s para %s: sua senha de acesso e %s (ZENVIA_API_TOKEN nao configurado, mensagem nao enviada de verdade)", channel, phone, password)
	return nil
}

// zenviaNotifier calls the Zenvia messaging API (https://zenvia.github.io/zenvia-openapi-spec/).
// Verify the request shape against Zenvia's current docs before relying on
// this in production — it was implemented without a live account to test
// against.
type zenviaNotifier struct {
	apiToken     string
	fromSMS      string
	fromWhatsApp string
	client       *http.Client
	baseURL      string
}

func (z *zenviaNotifier) SendPassword(ctx context.Context, phone, password, channel string) error {
	from := z.fromSMS
	if channel == ChannelWhatsApp {
		from = z.fromWhatsApp
	}
	text := fmt.Sprintf("BarberFlow: sua senha de acesso e %s", password)
	body, err := json.Marshal(map[string]any{
		"from": from,
		"to":   phone,
		"contents": []map[string]string{
			{"type": "text", "text": text},
		},
	})
	if err != nil {
		return err
	}
	url := fmt.Sprintf("%s/v2/channels/%s/messages", z.baseURL, channel)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-TOKEN", z.apiToken)

	resp, err := z.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("zenvia: unexpected status %d", resp.StatusCode)
	}
	return nil
}
