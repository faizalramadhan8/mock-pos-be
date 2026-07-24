package biteship

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
	http         *http.Client
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
		http:         &http.Client{Timeout: 15 * time.Second},
	}
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
		return nil, fmt.Errorf("biteship error (%d): %s", resp.StatusCode, string(respBody))
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
		return nil, err
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
