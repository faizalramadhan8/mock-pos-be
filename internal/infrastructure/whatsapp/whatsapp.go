// Package whatsapp wraps the WAHA (WhatsApp HTTP API) service used for
// sending receipt messages to members after checkout.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

type Service struct {
	receiptEnabled bool
	configured     bool
	baseURL        string
	apiKey         string
	session        string
	client         *http.Client
	log            *zerolog.Logger
}

func New(baseURL, apiKey, session string, receiptEnabled bool, log *zerolog.Logger) *Service {
	configured := baseURL != "" && apiKey != ""
	return &Service{
		receiptEnabled: receiptEnabled && configured,
		configured:     configured,
		baseURL:        strings.TrimRight(baseURL, "/"),
		apiKey:         apiKey,
		session:        session,
		client:         &http.Client{Timeout: 10 * time.Second},
		log:            log,
	}
}

// Enabled reports whether receipt sending is configured and turned on.
func (s *Service) Enabled() bool { return s.receiptEnabled }

// Configured reports whether WAHA is reachable (baseURL + apiKey set),
// regardless of the receipt toggle. Used by security notifications which
// must always fire independent of WA_RECEIPT_ENABLED.
func (s *Service) Configured() bool { return s.configured }

// normalizePhone takes a raw phone (e.g. "08123456789", "+628123...", "628...")
// and returns the WAHA chatId format "628xxx@c.us". Returns empty string on
// inputs that can't be normalized to an Indonesian mobile number.
func normalizePhone(raw string) string {
	// Keep digits only
	digits := regexp.MustCompile(`\D`).ReplaceAllString(raw, "")
	if digits == "" {
		return ""
	}
	// Convert leading 0 → 62 (Indonesia)
	switch {
	case strings.HasPrefix(digits, "62"):
		// already E.164 without +
	case strings.HasPrefix(digits, "0"):
		digits = "62" + digits[1:]
	default:
		// assume already international number without + or 0
		digits = "62" + digits
	}
	if len(digits) < 10 || len(digits) > 15 {
		return ""
	}
	return digits + "@c.us"
}

type sendTextReq struct {
	ChatID  string `json:"chatId"`
	Text    string `json:"text"`
	Session string `json:"session"`
}

// SendText sends a plain text message to the given phone number, gated by the
// receipt toggle (WA_RECEIPT_ENABLED). No-op when disabled. Errors are not
// returned as hard failures; callers should not block checkout flow on delivery.
func (s *Service) SendText(ctx context.Context, phone, text string) error {
	if !s.receiptEnabled {
		return nil
	}
	return s.send(ctx, phone, text)
}

// SendSecurityText sends a message bypassing the receipt toggle — used for
// device-approval notifications that must fire even when receipts are off.
// Still a no-op when WAHA itself is not configured.
func (s *Service) SendSecurityText(ctx context.Context, phone, text string) error {
	if !s.configured {
		return nil
	}
	return s.send(ctx, phone, text)
}

func (s *Service) send(ctx context.Context, phone, text string) error {
	chatID := normalizePhone(phone)
	if chatID == "" {
		return fmt.Errorf("invalid phone number: %q", phone)
	}

	body, err := json.Marshal(sendTextReq{ChatID: chatID, Text: text, Session: s.session})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+"/api/sendText", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Api-Key", s.apiKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("waha send failed: status %d", resp.StatusCode)
	}
	return nil
}
