package dto

import "encoding/json"

// EcomAdminProductResponse — data produk untuk admin ecom edit page.
// Include POS-side fields (stock_pos, selling_price, member_price) sebagai
// READ-ONLY context. Admin ecom lihat baseline POS tapi tidak bisa edit dari
// panel ecom.
type EcomAdminProductResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameID string `json:"name_id"`
	SKU    string `json:"sku"`

	// POS fields (read-only info)
	StockPOS     int      `json:"stock_pos"`
	SellingPrice float64  `json:"selling_price"`
	MemberPrice  *float64 `json:"member_price,omitempty"`
	Image        string   `json:"image,omitempty"`

	// Ecom fields (editable via PATCH /ecom-fields)
	StockEcom       int      `json:"stock_ecom"`
	EcomPrice       *float64 `json:"ecom_price"`
	EcomMemberPrice *float64 `json:"ecom_member_price"`
	EcomIsAvailable bool     `json:"ecom_is_available"`
	EcomDescription *string  `json:"ecom_description"`
	EcomImage       *string  `json:"ecom_image"`
	EcomCategoryID  *string  `json:"ecom_category_id"`
	EcomCategoryName string  `json:"ecom_category_name,omitempty"`
	EcomWeightGrams *int     `json:"ecom_weight_grams"`
	EcomMinOrder    int      `json:"ecom_min_order"`
}

// NullableInt — helper untuk distinguish "field tidak dikirim" (nil pointer)
// vs "field dikirim NULL" (pointer ke NullableInt dengan Null=true).
// Semantik penting untuk PATCH: kalau user kosongkan input ecom_price, kita
// mau SET NULL (fallback ke selling_price). Kalau field tidak dikirim di
// payload, kita tidak update.
type NullableInt struct {
	Value int  `json:"value"`
	Null  bool `json:"null"`
}

type NullableFloat struct {
	Value float64 `json:"value"`
	Null  bool    `json:"null"`
}

type NullableString struct {
	Value string `json:"value"`
	Null  bool   `json:"null"`
}

// UnmarshalJSON custom untuk parse: number → {Value:n, Null:false}, null →
// {Null:true}. Cegah FE harus kirim {"value": 0, "null": true} verbose.
func (n *NullableFloat) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Null = true
		return nil
	}
	return json.Unmarshal(data, &n.Value)
}
func (n *NullableInt) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Null = true
		return nil
	}
	return json.Unmarshal(data, &n.Value)
}
func (n *NullableString) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		n.Null = true
		return nil
	}
	return json.Unmarshal(data, &n.Value)
}

// EcomCategoryRequest — CRUD payload untuk admin. Response tipe di
// ecom_public.go (dipakai bareng admin + public).
type EcomCategoryRequest struct {
	Name      string `json:"name"`
	NameID    string `json:"name_id"`
	IconName  string `json:"icon_name"`
	SortOrder int    `json:"sort_order"`
	IsActive  *bool  `json:"is_active,omitempty"`
}

// EcomFieldsUpdateRequest — PATCH payload. Semua field pointer supaya
// FE bisa partial update. NullableX untuk distinguish "clear ke NULL" vs
// "tidak update".
type EcomFieldsUpdateRequest struct {
	StockEcom       *int            `json:"stock_ecom,omitempty"`
	EcomPrice       *NullableFloat  `json:"ecom_price,omitempty"`
	EcomMemberPrice *NullableFloat  `json:"ecom_member_price,omitempty"`
	EcomIsAvailable *bool           `json:"ecom_is_available,omitempty"`
	EcomDescription *NullableString `json:"ecom_description,omitempty"`
	EcomImage       *NullableString `json:"ecom_image,omitempty"`
	EcomCategoryID  *NullableString `json:"ecom_category_id,omitempty"`
	EcomWeightGrams *NullableInt    `json:"ecom_weight_grams,omitempty"`
	EcomMinOrder    *int            `json:"ecom_min_order,omitempty"`
}
