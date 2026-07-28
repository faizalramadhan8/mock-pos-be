package midtrans

import (
	"bytes"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Midtrans Snap API client (Bu Santi 24 Jul 2026).
// Docs: https://docs.midtrans.com/reference/getting-started-snap-api
//
// Kalau ServerKey kosong = stub mode (return dummy token). Cocok untuk dev
// belum daftar Midtrans, tapi flow checkout tetap bisa di-test.

type Client struct {
	ServerKey string
	IsProd    bool
	http      *http.Client
}

func NewClient(serverKey string, isProd bool) *Client {
	return &Client{
		ServerKey: serverKey,
		IsProd:    isProd,
		http:      &http.Client{Timeout: 15 * time.Second},
	}
}

// IsConfigured — true kalau ServerKey set. Consumer bisa branch stub vs real.
func (c *Client) IsConfigured() bool { return c.ServerKey != "" }

func (c *Client) baseURL() string {
	if c.IsProd {
		return "https://app.midtrans.com/snap/v1"
	}
	return "https://app.sandbox.midtrans.com/snap/v1"
}

type ItemDetail struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Category string  `json:"category,omitempty"`
}

type CustomerDetail struct {
	FirstName string `json:"first_name,omitempty"`
	Email     string `json:"email,omitempty"`
	Phone     string `json:"phone,omitempty"`
}

type TransactionDetails struct {
	OrderID     string  `json:"order_id"`
	GrossAmount float64 `json:"gross_amount"`
}

type SnapRequest struct {
	TransactionDetails TransactionDetails `json:"transaction_details"`
	ItemDetails        []ItemDetail       `json:"item_details,omitempty"`
	CustomerDetails    *CustomerDetail    `json:"customer_details,omitempty"`
	// Enabled payment methods — Midtrans default: all. Pin dulu ke QRIS + VA
	// + ewallet + card supaya fokus payment yang common di Indonesia.
	EnabledPayments []string `json:"enabled_payments,omitempty"`
	// Expiry — auto-cancel kalau tidak bayar.
	Expiry *SnapExpiry `json:"expiry,omitempty"`
}

type SnapExpiry struct {
	StartTime string `json:"start_time"`
	Unit      string `json:"unit"`     // "hour" / "minute" / "day"
	Duration  int    `json:"duration"` // 24 hour default
}

type SnapResponse struct {
	Token       string   `json:"token"`
	RedirectURL string   `json:"redirect_url"`
	ErrorMsg    []string `json:"error_messages,omitempty"`
}

// CreateSnapToken — POST /snap/v1/transactions.
func (c *Client) CreateSnapToken(req SnapRequest) (*SnapResponse, error) {
	if !c.IsConfigured() {
		// Stub — return fake token untuk dev mode. Frontend akan handle: kalau
		// token dimulai dengan "stub-", skip Snap popup, langsung tampil
		// "Sedang menunggu konfirmasi" state.
		return &SnapResponse{
			Token:       "stub-" + req.TransactionDetails.OrderID,
			RedirectURL: "",
		}, nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequest("POST", c.baseURL()+"/transactions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	// Midtrans auth: Basic base64(SERVER_KEY:) — password kosong, colon
	// tetap ada. Standar HTTP Basic Auth.
	auth := base64.StdEncoding.EncodeToString([]byte(c.ServerKey + ":"))
	httpReq.Header.Set("Authorization", "Basic "+auth)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var snapResp SnapResponse
	if err := json.Unmarshal(respBody, &snapResp); err != nil {
		return nil, fmt.Errorf("midtrans decode: %w (body: %s)", err, string(respBody))
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("midtrans error (%d): %v", resp.StatusCode, snapResp.ErrorMsg)
	}
	return &snapResp, nil
}

// VerifySignature — cek signature_key dari webhook Midtrans authentic.
// Formula: SHA512(order_id + status_code + gross_amount + server_key)
// Docs: https://docs.midtrans.com/reference/http-notification-and-signature-key
//
// Return true kalau:
//   - Client belum configured (stub mode) — dev mode auto-pass
//   - Signature match
// Return false kalau live mode dan hash tidak match (potensi spoofing).
func (c *Client) VerifySignature(orderID, statusCode, grossAmount, signature string) bool {
	if !c.IsConfigured() {
		return true // stub mode — bypass, tidak ada real signature dari sandbox
	}
	raw := orderID + statusCode + grossAmount + c.ServerKey
	sum := sha512.Sum512([]byte(raw))
	expected := hex.EncodeToString(sum[:])
	return expected == signature
}

// Notification — payload dari webhook. Handler subset — Midtrans kirim banyak
// field, kita hanya butuh identifier + status.
type Notification struct {
	TransactionTime   string `json:"transaction_time"`
	TransactionStatus string `json:"transaction_status"` // settlement, capture, pending, expire, cancel, deny
	TransactionID     string `json:"transaction_id"`
	StatusCode        string `json:"status_code"`
	PaymentType       string `json:"payment_type"`
	OrderID           string `json:"order_id"`
	SignatureKey      string `json:"signature_key"`
	FraudStatus       string `json:"fraud_status,omitempty"`
	SettlementTime    string `json:"settlement_time,omitempty"`
	GrossAmount       string `json:"gross_amount"`
}
