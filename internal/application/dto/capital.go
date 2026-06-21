package dto

type CapitalInjectionResponse struct {
	ID         string  `json:"id"`
	Amount     float64 `json:"amount"`
	Source     string  `json:"source,omitempty"`
	Note       string  `json:"note,omitempty"`
	InjectedAt string  `json:"injected_at"`
	CreatedBy  *string `json:"created_by,omitempty"`
	CreatedAt  string  `json:"created_at"`
}

type SaveCapitalInjectionRequest struct {
	Amount     float64 `json:"amount" validate:"required,gt=0"`
	Source     string  `json:"source"`
	Note       string  `json:"note"`
	InjectedAt string  `json:"injected_at" validate:"required"` // YYYY-MM-DD atau RFC3339
}
