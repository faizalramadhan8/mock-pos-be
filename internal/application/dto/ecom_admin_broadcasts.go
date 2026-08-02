package dto

// Sprint 5 Chunk 6 (2 Aug 2026) — Broadcast push DTOs.

type EcomBroadcastCreateRequest struct {
	Title string `json:"title" validate:"required,min=3,max=100"`
	Body  string `json:"body"  validate:"required,min=3,max=500"`
	// URL yang customer buka saat klik notif. Bisa relatif (/produk/xxx)
	// atau full URL. Opsional — kalau kosong customer buka storefront root.
	URL string `json:"url"`
}

type EcomBroadcastResponse struct {
	ID               string `json:"id"`
	Title            string `json:"title"`
	Body             string `json:"body"`
	URL              string `json:"url,omitempty"`
	DeliveredCount   int    `json:"delivered_count"`
	FailedCount      int    `json:"failed_count"`
	TotalSubscribers int    `json:"total_subscribers"`
	SentBy           string `json:"sent_by"`
	SentByName       string `json:"sent_by_name,omitempty"`
	SentAt           string `json:"sent_at"`
}
