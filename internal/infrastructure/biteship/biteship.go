package biteship

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Biteship shipping rate client — matches Postman collection reference
// (Rates API By Area Id / By Postal Code / By Mix).
// Docs: https://biteship.com/id/docs/api/rates
//
// Configured = punya APIKey. Origin bisa area_id, postal_code, atau keduanya
// (Biteship pakai area_id kalau ada, else fallback ke postal_code).
// Kalau APIKey kosong → stub mode (flat rate per kg).

type Client struct {
	APIKey       string
	OriginArea   string
	OriginPostal string
	Couriers     string // comma-separated: "jne,jnt,sicepat"
	FlatBase     int    // fallback stub Rp/kg
	// Origin (shipper) info untuk Create Order API (bukan cuma rate lookup).
	OriginName    string
	OriginPhone   string
	OriginAddress string
	OriginNote    string
	// Webhook shared secret (untuk verify signature). Kosong = skip verify.
	WebhookSecret string
	http          *http.Client
}

func NewClient(apiKey, originArea, originPostal, couriers string, flatBase int) *Client {
	if flatBase <= 0 {
		flatBase = 15000
	}
	if couriers == "" {
		couriers = "jne,jnt,sicepat,anteraja"
	}
	return &Client{
		APIKey:       apiKey,
		OriginArea:   originArea,
		OriginPostal: originPostal,
		Couriers:     couriers,
		FlatBase:     flatBase,
		http:         &http.Client{Timeout: 20 * time.Second},
	}
}

// WithShipper — chainable setter untuk info origin. Dipisah dari NewClient
// biar backward compat + arg count NewClient tidak meledak.
func (c *Client) WithShipper(name, phone, address, note, webhookSecret string) *Client {
	c.OriginName = name
	c.OriginPhone = phone
	c.OriginAddress = address
	c.OriginNote = note
	c.WebhookSecret = webhookSecret
	return c
}

// CanCreateOrder — semua field wajib untuk POST /orders sudah di-set.
// Rate API cuma butuh area/postal + items; Order API butuh full shipper info +
// destination contact/address.
func (c *Client) CanCreateOrder() bool {
	return c.IsConfigured() && c.OriginName != "" && c.OriginPhone != "" && c.OriginAddress != ""
}

// IsConfigured — punya API key + minimal salah satu origin identifier.
func (c *Client) IsConfigured() bool {
	return c.APIKey != "" && (c.OriginArea != "" || c.OriginPostal != "")
}

// Rate — 1 shipping option (kurir × service).
type Rate struct {
	Courier     string  `json:"courier"`
	CourierName string  `json:"courier_name"`
	Service     string  `json:"service"`
	ServiceName string  `json:"service_name"`
	Cost        float64 `json:"cost"`
	ETD         string  `json:"etd"`
}

// Item — matches Postman rate item shape. Field wajib untuk pricing akurat:
// weight (gram), quantity, value (Rp). Length/width/height opsional untuk
// kurir yang hitung volumetric weight.
type Item struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Value       int    `json:"value"`    // harga per unit (Rp)
	Weight      int    `json:"weight"`   // per unit (gram)
	Quantity    int    `json:"quantity"`
	Length      int    `json:"length,omitempty"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
}

type RateRequest struct {
	DestinationAreaID   string
	DestinationPostal   string
	DestinationLat      *float64
	DestinationLng      *float64
	Items               []Item
	// TotalWeightGrams — dipakai stub fallback. Real Biteship pakai items[].weight.
	TotalWeightGrams    int
}

// GetRates — Biteship POST /v1/rates/couriers. Kirim origin+destination
// pakai area_id kalau ada, fallback postal_code, fallback coordinates
// (untuk instant courier). Prioritas: area_id > postal_code > coordinates.
func (c *Client) GetRates(req RateRequest) ([]Rate, error) {
	if !c.IsConfigured() {
		return c.stubRates(req.TotalWeightGrams), nil
	}

	body := map[string]interface{}{
		"couriers": c.Couriers,
		"items":    c.itemsForRequest(req),
	}

	// Origin — prefer area_id, fallback postal_code.
	if c.OriginArea != "" {
		body["origin_area_id"] = c.OriginArea
	} else if c.OriginPostal != "" {
		body["origin_postal_code"] = c.OriginPostal
	}

	// Destination — prefer area_id, fallback postal_code, fallback coordinates.
	switch {
	case req.DestinationAreaID != "":
		body["destination_area_id"] = req.DestinationAreaID
	case req.DestinationPostal != "":
		body["destination_postal_code"] = req.DestinationPostal
	case req.DestinationLat != nil && req.DestinationLng != nil:
		body["destination_latitude"] = *req.DestinationLat
		body["destination_longitude"] = *req.DestinationLng
	default:
		// Tidak cukup info destination — fallback stub.
		return c.stubRates(req.TotalWeightGrams), nil
	}

	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequest("POST", "https://api.biteship.com/v1/rates/couriers", bytes.NewReader(buf))
	if err != nil {
		return c.stubRates(req.TotalWeightGrams), nil
	}
	httpReq.Header.Set("Authorization", c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		// Network down — jangan block checkout; fallback stub.
		return c.stubRates(req.TotalWeightGrams), nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		// Biteship reject request (400 area/postal invalid, 401 key salah,
		// 429 rate limit, 5xx down). Jangan fail checkout — fallback stub +
		// log detail supaya operator bisa debug.
		log.Printf("biteship rates error (%d): %s", resp.StatusCode, string(respBody))
		return c.stubRates(req.TotalWeightGrams), nil
	}

	var raw struct {
		Success bool `json:"success"`
		Pricing []struct {
			Courier     string  `json:"courier_code"`
			CourierName string  `json:"courier_name"`
			Service     string  `json:"courier_service_code"`
			ServiceName string  `json:"courier_service_name"`
			Price       float64 `json:"price"`
			ETD         string  `json:"duration"`
		} `json:"pricing"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		log.Printf("biteship rates decode error: %v", err)
		return c.stubRates(req.TotalWeightGrams), nil
	}
	rates := make([]Rate, 0, len(raw.Pricing))
	for _, p := range raw.Pricing {
		rates = append(rates, Rate{
			Courier: p.Courier, CourierName: p.CourierName,
			Service: p.Service, ServiceName: p.ServiceName,
			Cost: p.Price, ETD: p.ETD,
		})
	}
	if len(rates) == 0 {
		return c.stubRates(req.TotalWeightGrams), nil
	}
	return rates, nil
}

// itemsForRequest — kalau caller supply real items pakai itu, else buat
// 1 aggregated line item pakai total weight (backward compat).
func (c *Client) itemsForRequest(req RateRequest) []map[string]interface{} {
	if len(req.Items) > 0 {
		out := make([]map[string]interface{}, 0, len(req.Items))
		for _, it := range req.Items {
			m := map[string]interface{}{
				"name":     it.Name,
				"value":    it.Value,
				"weight":   it.Weight,
				"quantity": it.Quantity,
			}
			if it.Description != "" {
				m["description"] = it.Description
			}
			if it.Length > 0 {
				m["length"] = it.Length
			}
			if it.Width > 0 {
				m["width"] = it.Width
			}
			if it.Height > 0 {
				m["height"] = it.Height
			}
			out = append(out, m)
		}
		return out
	}
	w := req.TotalWeightGrams
	if w < 1 {
		w = 1000
	}
	return []map[string]interface{}{
		{
			"name":     "Bahan Kue",
			"value":    10000,
			"weight":   w,
			"quantity": 1,
		},
	}
}

// stubRates — flat rate fallback ketika Biteship tidak configured / down.
func (c *Client) stubRates(weightGrams int) []Rate {
	kg := (weightGrams + 999) / 1000
	if kg < 1 {
		kg = 1
	}
	base := c.FlatBase * kg
	return []Rate{
		{Courier: "jne", CourierName: "JNE", Service: "REG", ServiceName: "Reguler", Cost: float64(base), ETD: "2-3 hari"},
		{Courier: "jne", CourierName: "JNE", Service: "YES", ServiceName: "YES (Next Day)", Cost: float64(base) * 1.5, ETD: "1 hari"},
		{Courier: "jnt", CourierName: "J&T Express", Service: "REG", ServiceName: "Reguler", Cost: float64(base - 2000), ETD: "2-3 hari"},
		{Courier: "sicepat", CourierName: "SiCepat", Service: "REG", ServiceName: "Reguler", Cost: float64(base - 1000), ETD: "2-4 hari"},
	}
}

// ─── Order API (Sprint 3) ────────────────────────────────────────────

// CreateOrderRequest — payload internal. Matches Postman "Order API Create
// By postal code" pattern. Destination info diisi caller dari order shipping
// address; origin diisi dari config toko.
type CreateOrderRequest struct {
	// Destination
	DestinationName    string
	DestinationPhone   string
	DestinationEmail   string
	DestinationAddress string
	DestinationPostal  string
	DestinationAreaID  string
	DestinationNote    string
	// Courier picked oleh customer di checkout
	CourierCode    string // "jne"
	CourierService string // "reg" / "yes"
	// Items snapshot (untuk kurir hitung insurance)
	Items []Item
	// Optional
	OrderNote        string
	CourierInsurance int
	// Tag untuk cross-reference dari webhook back ke order.id kita.
	// Biteship echo balik di response + notif via `reference_id`.
	ReferenceID string
}

// CreateOrderResponse — subset field yang kita butuh dari Biteship response.
type CreateOrderResponse struct {
	OrderID     string `json:"id"`
	Status      string `json:"status"`
	CourierCode string `json:"-"` // populated dari nested pricing
	Waybill     string `json:"-"` // biasanya baru terisi setelah pickup — kosong di awal
	Price       int    `json:"-"` // final ongkir dari Biteship
}

// CreateOrder — POST /v1/orders. Return order_id + status. Waybill (nomor
// resi) biasanya belum ada di respon awal — masuk lewat webhook `order.waybill_id`.
func (c *Client) CreateOrder(req CreateOrderRequest) (*CreateOrderResponse, error) {
	if !c.CanCreateOrder() {
		return nil, fmt.Errorf("biteship: origin shipper info belum lengkap (name/phone/address)")
	}

	body := map[string]interface{}{
		"shipper_contact_name":       c.OriginName,
		"shipper_contact_phone":      c.OriginPhone,
		"origin_contact_name":        c.OriginName,
		"origin_contact_phone":       c.OriginPhone,
		"origin_address":             c.OriginAddress,
		"destination_contact_name":   req.DestinationName,
		"destination_contact_phone":  req.DestinationPhone,
		"destination_address":        req.DestinationAddress,
		"courier_company":            req.CourierCode,
		"courier_type":               req.CourierService,
		"delivery_type":              "now",
		"items":                      c.itemsForRequest(RateRequest{Items: req.Items}),
	}
	if c.OriginNote != "" {
		body["origin_note"] = c.OriginNote
	}
	if req.DestinationEmail != "" {
		body["destination_contact_email"] = req.DestinationEmail
	}
	if req.DestinationNote != "" {
		body["destination_note"] = req.DestinationNote
	}
	if req.OrderNote != "" {
		body["order_note"] = req.OrderNote
	}
	if req.CourierInsurance > 0 {
		body["courier_insurance"] = req.CourierInsurance
	}
	if req.ReferenceID != "" {
		body["reference_id"] = req.ReferenceID
	}
	// Origin — prefer area_id; else postal.
	if c.OriginArea != "" {
		body["origin_area_id"] = c.OriginArea
	} else if c.OriginPostal != "" {
		body["origin_postal_code"] = c.OriginPostal
	}
	// Destination — prefer area_id; else postal.
	if req.DestinationAreaID != "" {
		body["destination_area_id"] = req.DestinationAreaID
	} else if req.DestinationPostal != "" {
		body["destination_postal_code"] = req.DestinationPostal
	} else {
		return nil, fmt.Errorf("biteship: destination area_id atau postal_code wajib")
	}

	buf, _ := json.Marshal(body)
	httpReq, err := http.NewRequest("POST", "https://api.biteship.com/v1/orders", bytes.NewReader(buf))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Authorization", c.APIKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("biteship create order (%d): %s", resp.StatusCode, string(respBody))
	}

	// Response Biteship punya struktur nested — kita cuma butuh id + status.
	var raw struct {
		Success bool   `json:"success"`
		Object  string `json:"object"`
		ID      string `json:"id"`
		Status  string `json:"status"`
		Courier struct {
			WaybillID string `json:"waybill_id"`
			Company   string `json:"company"`
			Type      string `json:"type"`
		} `json:"courier"`
		Price int `json:"price"`
	}
	if err := json.Unmarshal(respBody, &raw); err != nil {
		return nil, fmt.Errorf("biteship decode: %w (body: %s)", err, string(respBody))
	}
	if !raw.Success && raw.ID == "" {
		return nil, fmt.Errorf("biteship response tidak sukses: %s", string(respBody))
	}
	return &CreateOrderResponse{
		OrderID:     raw.ID,
		Status:      raw.Status,
		CourierCode: raw.Courier.Company,
		Waybill:     raw.Courier.WaybillID,
		Price:       raw.Price,
	}, nil
}

// CancelOrder — POST /v1/orders/:id/cancel. Dipanggil kalau admin batalkan
// order yang sudah terlanjur di-create di Biteship (belum di-pickup kurir).
func (c *Client) CancelOrder(biteshipOrderID string) error {
	if !c.IsConfigured() {
		return nil // silent no-op — order tidak pernah di-create di real Biteship
	}
	httpReq, err := http.NewRequest("POST", "https://api.biteship.com/v1/orders/"+biteshipOrderID+"/cancel", nil)
	if err != nil {
		return err
	}
	httpReq.Header.Set("Authorization", c.APIKey)
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("biteship cancel (%d): %s", resp.StatusCode, string(b))
	}
	return nil
}

// VerifyWebhookSignature — HMAC-SHA256 verify header sesuai config webhook di
// Biteship dashboard. Kalau WebhookSecret kosong = auto-pass (dev mode).
//
// Biteship header format (per docs): `X-Biteship-Signature: <hmac-sha256-hex>`
// dengan body raw sebagai message. Beberapa versi Biteship pakai header lain —
// caller kirim signature header apapun yang di-terima, kita compare.
func (c *Client) VerifyWebhookSignature(body []byte, signature string) bool {
	if c.WebhookSecret == "" {
		return true
	}
	mac := hmac.New(sha256.New, []byte(c.WebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}
