package account

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/avdoseferovic/geoserv/internal/config"
)

var ErrEmailTransportUnavailable = errors.New("email transport unavailable")

type Sender interface {
	Status() SenderStatus
	SendAccountValidation(context.Context, ValidationEmail) error
	SendRecoveryPIN(context.Context, RecoveryEmail) error
}

type SenderStatus struct {
	Configured bool
	Ready      bool
	Reason     string
}

type ValidationEmail struct {
	AccountName string
	Email       string
}

type RecoveryEmail struct {
	AccountName string
	Email       string
	PIN         string
	ExpiresIn   time.Duration
}

type noopSender struct {
	status SenderStatus
}

func NewSender(cfg *config.Config) Sender {
	status := senderStatus(cfg)
	if !status.Configured {
		return noopSender{status: status}
	}
	return smtpSender{cfg: cfg, status: status}
}

func (s noopSender) Status() SenderStatus {
	return s.status
}

func (s noopSender) SendAccountValidation(context.Context, ValidationEmail) error {
	return fmt.Errorf("%w: %s", ErrEmailTransportUnavailable, s.status.Reason)
}

func (s noopSender) SendRecoveryPIN(context.Context, RecoveryEmail) error {
	return fmt.Errorf("%w: %s", ErrEmailTransportUnavailable, s.status.Reason)
}

func senderStatus(cfg *config.Config) SenderStatus {
	if cfg == nil {
		return SenderStatus{Reason: "server config is unavailable"}
	}

	if cfg.SMTP.Host == "" || cfg.SMTP.Port <= 0 || cfg.SMTP.FromAddress == "" {
		return SenderStatus{Reason: "SMTP is not configured"}
	}

	return SenderStatus{
		Configured: true,
		Ready:      true,
		Reason:     "SMTP is configured",
	}
}

type smtpSender struct {
	cfg    *config.Config
	status SenderStatus
}

func (s smtpSender) Status() SenderStatus {
	return s.status
}

func (s smtpSender) SendAccountValidation(ctx context.Context, email ValidationEmail) error {
	body := fmt.Sprintf("Hello %s,\n\nYour account was created successfully.\n", email.AccountName)
	return s.send(ctx, email.Email, email.AccountName, "Account validation", body)
}

func (s smtpSender) SendRecoveryPIN(ctx context.Context, email RecoveryEmail) error {
	body := fmt.Sprintf(
		"Hello %s,\n\nYour recovery PIN is %s.\nIt expires in %s.\n",
		email.AccountName,
		email.PIN,
		email.ExpiresIn.Round(time.Minute),
	)
	return s.send(ctx, email.Email, email.AccountName, "Recovery PIN", body)
}

func (s smtpSender) send(ctx context.Context, toAddress, toName, subject, body string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	fromHeader := s.cfg.SMTP.FromAddress
	if strings.TrimSpace(s.cfg.SMTP.FromName) != "" {
		fromHeader = fmt.Sprintf("%s <%s>", s.cfg.SMTP.FromName, s.cfg.SMTP.FromAddress)
	}
	toHeader := toAddress
	if strings.TrimSpace(toName) != "" {
		toHeader = fmt.Sprintf("%s <%s>", toName, toAddress)
	}

	message := strings.Join([]string{
		"From: " + fromHeader,
		"To: " + toHeader,
		"Subject: " + subject,
		"MIME-Version: 1.0",
		"Content-Type: text/plain; charset=UTF-8",
		"",
		body,
	}, "\r\n")

	var auth smtp.Auth
	if s.cfg.SMTP.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.SMTP.Username, s.cfg.SMTP.Password, s.cfg.SMTP.Host)
	}

	return smtp.SendMail(s.cfg.SMTP.Address(), auth, s.cfg.SMTP.FromAddress, []string{toAddress}, []byte(message))
}
