package dto

// EcomCategoryResponse — kategori untuk storefront browse + admin list.
// Migration 000053: switched dari POS categories ke ecom_categories terpisah.
// icon_name = kolom di DB. sort_order dari admin. is_active = admin toggle.
// ProductCount: admin scope = semua produk ter-link, public = filter eligible.
type EcomCategoryResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	NameID       string `json:"name_id"`
	IconName     string `json:"icon_name,omitempty"`
	SortOrder    int    `json:"sort_order"`
	IsActive     bool   `json:"is_active"`
	ProductCount int    `json:"product_count"`
}

// EcomProductListItem — compact untuk grid + card. Deskripsi + tier tidak
// dikirim di list (hemat bandwidth). Detail via GET /:id.
type EcomProductListItem struct {
	ID              string   `json:"id"`
	Name            string   `json:"name"`
	NameID          string   `json:"name_id"`
	SKU             string   `json:"sku"`
	CategoryID      string   `json:"category_id"`
	CategoryName    string   `json:"category_name,omitempty"`
	Image           string   `json:"image,omitempty"`
	Price           float64  `json:"price"`             // ecom_price fallback selling_price
	MemberPrice     *float64 `json:"member_price,omitempty"` // ecom_member_price fallback member_price
	Stock           int      `json:"stock"`             // stock_ecom
	WeightGrams     *int     `json:"weight_grams,omitempty"`
	MinOrder        int      `json:"min_order"`
	IsLowStock      bool     `json:"is_low_stock"`      // stock ≤ 5 (urgency signal)
}

// EcomProductDetail — full detail untuk PDP. Include description + tiers +
// gallery images. Image utama tetap di parent (EcomProductListItem.Image);
// Images[0] biasanya = Image (duplicate OK — FE bisa dedup atau tampil
// semua). Kalau EcomImages kosong di DB, Images = [Image] saja (fallback).
type EcomProductDetail struct {
	EcomProductListItem
	Description string                 `json:"description,omitempty"`
	Images      []string               `json:"images,omitempty"` // gallery untuk swipe
	Tiers       []EcomProductTierPrice `json:"tiers,omitempty"`
}

// EcomProductTierPrice — grosir toggle. Only tier target='all_customers'
// yang tampil di public storefront (customer non-member context).
type EcomProductTierPrice struct {
	MinQty int     `json:"min_qty"`
	Price  float64 `json:"price"` // per-satuan (sudah dibagi min_qty saat storage)
	Note   string  `json:"note,omitempty"`
}

// EcomProductListResponse — cursor pagination wrapper.
type EcomProductListResponse struct {
	Items      []EcomProductListItem `json:"items"`
	NextCursor string                `json:"next_cursor"`
	Total      int64                 `json:"total,omitempty"`
}
