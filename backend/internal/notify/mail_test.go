package notify

import (
	"context"
	"errors"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestNewMailer_NoHostReturnsLogMailer(t *testing.T) {
	if _, ok := NewMailer(SMTPConfig{}).(logMailer); !ok {
		t.Fatal("expected logMailer without SMTP host")
	}
	if _, ok := NewMailer(SMTPConfig{Host: "smtp.example.com"}).(*smtpMailer); !ok {
		t.Fatal("expected smtpMailer with SMTP host")
	}
}

func TestBuildPasswordEmail(t *testing.T) {
	msg := string(BuildPasswordEmail("barbearia@example.com", "ana@example.com", "Ana", "123456", "https://app.example.com/junin", time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)))
	for _, want := range []string{
		"From: barbearia@example.com\r\n",
		"To: ana@example.com\r\n",
		"Subject: Seu acesso ao BarberFlow\r\n",
		"Content-Type: text/plain; charset=utf-8\r\n",
		"\r\n\r\nOlá, Ana!",
		"Endereço: https://app.example.com/junin",
		"Senha temporária: 123456",
		"Perfil > Alterar senha",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestSMTPMailer_SendsToConfiguredServer(t *testing.T) {
	var gotAddr, gotFrom string
	var gotTo []string
	var gotAuth smtp.Auth
	mailer := &smtpMailer{
		cfg: SMTPConfig{Host: "smtp.example.com", Port: "2525", Username: "user", Password: "pass", From: "no-reply@example.com"},
		send: func(addr string, a smtp.Auth, from string, to []string, _ []byte) error {
			gotAddr, gotAuth, gotFrom, gotTo = addr, a, from, to
			return nil
		},
	}
	if err := mailer.SendPasswordEmail(context.Background(), "ana@example.com", "Ana", "123456", "https://x/junin"); err != nil {
		t.Fatalf("send: %v", err)
	}
	if gotAddr != "smtp.example.com:2525" || gotFrom != "no-reply@example.com" || len(gotTo) != 1 || gotTo[0] != "ana@example.com" || gotAuth == nil {
		t.Fatalf("unexpected send call: %s %s %v %v", gotAddr, gotFrom, gotTo, gotAuth)
	}

	mailer.send = func(string, smtp.Auth, string, []string, []byte) error { return errors.New("connection refused") }
	if err := mailer.SendPasswordEmail(context.Background(), "ana@example.com", "Ana", "1", "u"); err == nil {
		t.Fatal("expected the SMTP error to surface")
	}
}
