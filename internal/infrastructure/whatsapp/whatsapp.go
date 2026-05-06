// Package whatsapp wraps the WAHA (WhatsApp HTTP API) service used for
// sending receipt messages to members after checkout.
package whatsapp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"
)

// Per-recipient rate limit — minimum gap antar pesan ke chatId yang sama.
// WAHA docs: 30-60 detik antar pesan ke contact yang sama. Pakai 30s
// supaya tidak terlalu lambat untuk multi-step workflow (struk + invoice).
const minSendGapPerContact = 30 * time.Second

type Service struct {
	enabled bool
	baseURL string
	apiKey  string
	session string
	client  *http.Client
	log     *zerolog.Logger

	// Per-recipient last-send tracker. Cegah burst ke 1 nomor; sesuai WAHA
	// rule: max 4 pesan/jam per contact, jeda 30-60s antar pesan.
	mu       sync.Mutex
	lastSent map[string]time.Time
}

func New(baseURL, apiKey, session string, enabled bool, log *zerolog.Logger) *Service {
	return &Service{
		enabled:  enabled && baseURL != "" && apiKey != "",
		baseURL:  strings.TrimRight(baseURL, "/"),
		apiKey:   apiKey,
		session:  session,
		client:   &http.Client{Timeout: 15 * time.Second},
		log:      log,
		lastSent: make(map[string]time.Time),
	}
}

// Enabled reports whether WA sending is configured and turned on. All WA
// emissions (receipts + security/transaction notifications) share this flag.
func (s *Service) Enabled() bool { return s.enabled }

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

type chatActionReq struct {
	ChatID  string `json:"chatId"`
	Session string `json:"session"`
}

// postJSON helper — POST request dengan body JSON ke WAHA endpoint relatif.
// Tidak panic kalau gagal; return error untuk caller decide.
func (s *Service) postJSON(ctx context.Context, path string, body any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.baseURL+path, bytes.NewReader(buf))
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
		return fmt.Errorf("waha %s: status %d", path, resp.StatusCode)
	}
	return nil
}

// sendSeen/startTyping/stopTyping — best-effort, kalau gagal cuma log debug,
// tidak block sendText. WAHA docs: workflow proper untuk simulate human:
// sendSeen → startTyping → delay → stopTyping → sendText.
//
// Note: sendSeen mark incoming messages as read. Untuk POS yang outbound-only
// (kirim struk, kirim notif admin), endpoint ini biasanya no-op atau minor
// error kalau tidak ada pesan masuk untuk di-mark. Tetap dipanggil supaya
// compliant dengan WAHA recommended flow + recipient lihat "✓✓" biru kalau
// ada pesan tertinggal sebelumnya.
func (s *Service) sendSeen(ctx context.Context, chatID string) {
	if err := s.postJSON(ctx, "/api/sendSeen", chatActionReq{ChatID: chatID, Session: s.session}); err != nil {
		s.log.Debug().Err(err).Msg("waha sendSeen failed (non-blocking)")
	}
}

func (s *Service) startTyping(ctx context.Context, chatID string) {
	if err := s.postJSON(ctx, "/api/startTyping", chatActionReq{ChatID: chatID, Session: s.session}); err != nil {
		s.log.Debug().Err(err).Msg("waha startTyping failed (non-blocking)")
	}
}

func (s *Service) stopTyping(ctx context.Context, chatID string) {
	if err := s.postJSON(ctx, "/api/stopTyping", chatActionReq{ChatID: chatID, Session: s.session}); err != nil {
		s.log.Debug().Err(err).Msg("waha stopTyping failed (non-blocking)")
	}
}

// waitForRecipientGap menunggu sampai recipient eligible kirim ulang. Cegah
// burst ke 1 nomor (≥30s gap). Tidak menunggu pasti — kalau ctx ke-cancel,
// fungsi return tanpa error supaya outer caller masih bisa send (best-effort).
func (s *Service) waitForRecipientGap(ctx context.Context, chatID string) {
	s.mu.Lock()
	last, ok := s.lastSent[chatID]
	s.mu.Unlock()
	if !ok {
		return
	}
	wait := minSendGapPerContact - time.Since(last)
	if wait <= 0 {
		return
	}
	select {
	case <-time.After(wait):
	case <-ctx.Done():
	}
}

// typingDelay — durasi natural untuk simulate user mengetik. Length-aware:
// pesan panjang lebih lama. Min 1s, max 4s, plus jitter 0.5-1.5s.
func typingDelay(textLen int) time.Duration {
	// ~50ms per char, capped 1-4 detik base
	base := time.Duration(textLen) * 50 * time.Millisecond
	if base < 1*time.Second {
		base = 1 * time.Second
	}
	if base > 4*time.Second {
		base = 4 * time.Second
	}
	jitter := time.Duration(500+rand.Intn(1000)) * time.Millisecond
	return base + jitter
}

// SendText sends a plain text message to the given phone number, gated by
// WA_RECEIPT_ENABLED (+ WAHA configured). No-op when disabled. Errors are
// not returned as hard failures; callers should not block on delivery.
//
// Workflow lengkap sesuai WAHA recommendation:
//   1. Wait min 30s gap kalau pernah kirim ke recipient sama.
//   2. POST /api/sendSeen     (mark unread incoming sebagai read)
//   3. POST /api/startTyping  (mimic human composing)
//   4. Sleep 1-4s + jitter (length-aware)
//   5. POST /api/stopTyping
//   6. POST /api/sendText
//   7. Update last-sent timestamp untuk recipient
func (s *Service) SendText(ctx context.Context, phone, text string) error {
	if !s.enabled {
		return nil
	}
	chatID := normalizePhone(phone)
	if chatID == "" {
		return fmt.Errorf("invalid phone number: %q", phone)
	}

	// Step 1: per-recipient rate limit (cegah burst ke 1 nomor).
	s.waitForRecipientGap(ctx, chatID)

	// Step 2: mark seen — best-effort, biasanya no-op kalau outbound only.
	s.sendSeen(ctx, chatID)

	// Step 3-5: typing simulation. Best-effort; kalau ctx canceled, skip.
	s.startTyping(ctx, chatID)
	select {
	case <-time.After(typingDelay(len(text))):
	case <-ctx.Done():
	}
	s.stopTyping(ctx, chatID)

	// Step 6: actual send.
	err := s.postJSON(ctx, "/api/sendText", sendTextReq{ChatID: chatID, Text: text, Session: s.session})
	if err == nil {
		// Step 7: track last-sent only on success, supaya retry tidak ke-throttle.
		s.mu.Lock()
		s.lastSent[chatID] = time.Now()
		s.mu.Unlock()
	}
	return err
}
