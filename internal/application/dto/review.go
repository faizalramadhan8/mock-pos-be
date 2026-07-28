package dto

// ReviewPublicItem — 1 row review untuk display di PDP (public).
type ReviewPublicItem struct {
	ID        string `json:"id"`
	Rating    int    `json:"rating"`
	Comment   string `json:"comment,omitempty"`
	UserName  string `json:"user_name"` // sudah di-mask (Fai R., dst)
	CreatedAt string `json:"created_at"`
}

// ReviewSummary — aggregate stats per produk.
type ReviewSummary struct {
	Count        int         `json:"count"`
	Average      float64     `json:"average"`
	Distribution map[int]int `json:"distribution"` // {1: N, 2: N, ..., 5: N}
}

// ReviewListResponse — public endpoint response.
type ReviewListResponse struct {
	Items   []ReviewPublicItem `json:"items"`
	Summary ReviewSummary      `json:"summary"`
}

// ReviewSubmitRequest — customer POST /reviews. product_id from body.
type ReviewSubmitRequest struct {
	ProductID string `json:"product_id" validate:"required,uuid"`
	Rating    int    `json:"rating" validate:"required,min=1,max=5"`
	Comment   string `json:"comment" validate:"omitempty,max=1000"`
}

// ReviewCanReviewResponse — check eligibility + optional existing review.
type ReviewCanReviewResponse struct {
	CanReview bool               `json:"can_review"`
	MyReview  *ReviewPublicItem  `json:"my_review,omitempty"`
}
