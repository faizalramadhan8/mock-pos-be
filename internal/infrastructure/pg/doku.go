// Package pg — Payment Gateway client untuk alifworks PG wrapper (DOKU).
// Ganti Midtrans Snap sejak 28 Jul 2026 — Bu Santi request pakai PG ini karena
// support VA multi-bank + QRIS + e-wallet + credit card dalam satu integration
// tanpa perlu approval merchant DOKU sendiri.
//
// Docs: postman collection Payment Gateway v2 - DOKU (root repo).
// Endpoints:
//   GET  {base}/payment/api/v1/channels          — public, list channels
//   POST {base}/payment/api/v1/pay/direct        — Basic Auth, create payment
//   GET  {base}/payment/api/v1/pay/status?trxId= — Basic Auth, check status
//
// Webhook (DOKU notification) datang dengan payload berbeda dari Midtrans,
// lihat WebhookPayload di file ini + handler-nya di ecom_checkout_handler.go.
package pg

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Client struct {
	BaseURL      string
	AppKey       string
	AppSecret    string
	MerchantName string
	http         *http.Client
}

func NewClient(baseURL, appKey, appSecret, merchantName string) *Client {
	return &Client{
		BaseURL:      baseURL,
		AppKey:       appKey,
		AppSecret:    appSecret,
		MerchantName: merchantName,
		http:         &http.Client{Timeout: 20 * time.Second},
	}
}

// IsConfigured — true kalau BaseURL + AppKey + AppSecret semua terisi.
// Consumer bisa branch: stub (return fake payment_url) vs live.
func (c *Client) IsConfigured() bool {
	return c.BaseURL != "" && c.AppKey != "" && c.AppSecret != ""
}

// ─── Create Payment ──────────────────────────────────────────────────

type CreatePaymentItem struct {
	ProductID   string  `json:"product_id"`
	ProductCode string  `json:"product_code"`
	ProductName string  `json:"product_name"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

type CreatePaymentRequest struct {
	Channel        string              `json:"channel"`        // bca/bri/bni/mandiri/cimb/permata/qris/ovo/shopeepay/dana/credit_card
	TotalAmount    float64             `json:"total_amount"`
	TrxID          string              `json:"trx_id"`         // = order.id (kita reuse UUID)
	BillingEmail   string              `json:"billing_email"`
	BillingName    string              `json:"billing_name"`
	BillingPhone   string              `json:"billing_phone"`
	BillingAddress string              `json:"billing_address"`
	BillingID      string              `json:"billing_id,omitempty"`
	BillingUID     string              `json:"billing_uid,omitempty"`
	CallbackURL    string              `json:"callback_url"`   // webhook BE kita
	SuccessURL     string              `json:"success_url"`    // redirect FE setelah bayar
	Description    string              `json:"description"`
	Merchant       string              `json:"merchant"`       // samaran per Bu Santi request
	Remark         string              `json:"remark"`
	Item           []CreatePaymentItem `json:"item"`
}

type CreatePaymentResponse struct {
	PaymentID       string  `json:"payment_id"`
	RefNo           string  `json:"ref_no"`
	Amount          float64 `json:"amount"`
	Currency        string  `json:"currency"`
	PaymentChannel  string  `json:"payment_channel"`
	PaymentCode     string  `json:"payment_code"`
	PaymentGateway  string  `json:"payment_gateway"`
	PaymentStatus   string  `json:"payment_status"` // UNPAID / PAID / EXPIRED / FAILED / CANCELLED
	PaymentURL      string  `json:"payment_url"`
	PaymentAdminFee float64 `json:"payment_admin_fee"`
	Expired         string  `json:"expired"`
}

// pgEnvelope — semua PG endpoint bungkus body di envelope {code,body,message}.
// code=0 = success. Kita unmarshal Body sebagai json.RawMessage lalu decode
// ke target concrete type per endpoint.
type pgEnvelope struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Error   string          `json:"error"`
	Body    json.RawMessage `json:"body"`
}

// CreatePayment — POST /payment/api/v1/pay/direct. Return PG response +
// payment_url yang customer buka untuk bayar.
func (c *Client) CreatePayment(ctx context.Context, req CreatePaymentRequest) (*CreatePaymentResponse, error) {
	if !c.IsConfigured() {
		// Stub — return fake payment_url untuk dev mode. FE branch: kalau URL
		// dimulai "stub-" tampil "Sedang menunggu konfirmasi" state.
		return &CreatePaymentResponse{
			PaymentID:     "stub-" + req.TrxID,
			RefNo:         req.TrxID,
			Amount:        req.TotalAmount,
			Currency:      "IDR",
			PaymentChannel: req.Channel,
			PaymentCode:   req.Channel,
			PaymentStatus: "UNPAID",
			PaymentURL:    "stub-" + req.TrxID,
		}, nil
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.BaseURL+"/payment/api/v1/pay/direct", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setAuth(httpReq)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var env pgEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("pg create decode envelope: %w (raw: %s)", err, string(respBody))
	}
	if resp.StatusCode >= 400 || env.Code != 0 {
		return nil, fmt.Errorf("pg create failed (http=%d code=%d): %s", resp.StatusCode, env.Code, env.Message)
	}
	var out CreatePaymentResponse
	if err := json.Unmarshal(env.Body, &out); err != nil {
		return nil, fmt.Errorf("pg create decode body: %w", err)
	}
	return &out, nil
}

// ─── Check Status ────────────────────────────────────────────────────

type StatusResponse struct {
	PaymentID       string  `json:"payment_id"`
	RefNo           string  `json:"ref_no"`
	Amount          float64 `json:"amount"`
	PaymentChannel  string  `json:"payment_channel"`
	PaymentCode     string  `json:"payment_code"`
	PaymentStatus   string  `json:"payment_status"`
	BillingName     string  `json:"billing_name"`
	BillingEmail    string  `json:"billing_email"`
	Created         string  `json:"created"`
	Updated         string  `json:"updated"`
	Expired         string  `json:"expired"`
	PaymentPaidDate string  `json:"payment_paid_date"`
}

// CheckStatus — GET /payment/api/v1/pay/status?trxId=. Dipakai untuk polling
// fallback kalau webhook gagal (mis. Bu Santi tidak lihat status update).
func (c *Client) CheckStatus(ctx context.Context, trxID string) (*StatusResponse, error) {
	if !c.IsConfigured() {
		return &StatusResponse{RefNo: trxID, PaymentStatus: "UNPAID"}, nil
	}
	q := url.Values{}
	q.Set("trxId", trxID)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", c.BaseURL+"/payment/api/v1/pay/status?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(httpReq)
	httpReq.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)

	var env pgEnvelope
	if err := json.Unmarshal(respBody, &env); err != nil {
		return nil, fmt.Errorf("pg status decode envelope: %w", err)
	}
	if resp.StatusCode == 404 || env.Code == 404 {
		return nil, ErrTransactionNotFound
	}
	if resp.StatusCode >= 400 || env.Code != 0 {
		return nil, fmt.Errorf("pg status failed (http=%d code=%d): %s", resp.StatusCode, env.Code, env.Message)
	}
	var out StatusResponse
	if err := json.Unmarshal(env.Body, &out); err != nil {
		return nil, fmt.Errorf("pg status decode body: %w", err)
	}
	return &out, nil
}

var ErrTransactionNotFound = fmt.Errorf("pg: transaction not found")

// ─── Webhook payload ─────────────────────────────────────────────────

// WebhookPayload — DOKU-standard notification body. Subset field yang kita
// pakai. `order.invoice_number` = trxID = order.id di kita.
type WebhookPayload struct {
	Service struct {
		ID string `json:"id"`
	} `json:"service"`
	Acquirer struct {
		ID string `json:"id"`
	} `json:"acquirer"`
	Channel struct {
		ID string `json:"id"`
	} `json:"channel"`
	Transaction struct {
		Status            string `json:"status"` // SUCCESS / FAILED / VOIDED
		Date              string `json:"date"`
		OriginalRequestID string `json:"original_request_id"`
	} `json:"transaction"`
	Order struct {
		InvoiceNumber string  `json:"invoice_number"`
		Amount        float64 `json:"amount"`
		SessionID     string  `json:"session_id"`
	} `json:"order"`
	VirtualAccountInfo struct {
		VirtualAccountNumber string `json:"virtual_account_number"`
	} `json:"virtual_account_info"`
}

// ─── helpers ─────────────────────────────────────────────────────────

func (c *Client) setAuth(r *http.Request) {
	if c.AppKey == "" && c.AppSecret == "" {
		return
	}
	auth := base64.StdEncoding.EncodeToString([]byte(c.AppKey + ":" + c.AppSecret))
	r.Header.Set("Authorization", "Basic "+auth)
}

// IsStubPaymentURL — helper untuk FE-safe check. FE tidak perlu tahu prefix.
func IsStubPaymentURL(u string) bool {
	return len(u) >= 5 && u[:5] == "stub-"
}
