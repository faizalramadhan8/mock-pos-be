package dto

type CashbookOpeningBalanceResponse struct {
	ID        string  `json:"id"`
	Year      int     `json:"year"`
	Month     int     `json:"month"`
	Balance   float64 `json:"balance"`
	Note      string  `json:"note,omitempty"`
	CreatedBy string  `json:"created_by"`
	CreatedAt string  `json:"created_at"`
	UpdatedAt string  `json:"updated_at"`
}

type SetOpeningBalanceRequest struct {
	Year    int     `json:"year" validate:"required,min=2020,max=2100"`
	Month   int     `json:"month" validate:"required,min=1,max=12"`
	Balance float64 `json:"balance" validate:"min=0"`
	Note    string  `json:"note,omitempty"`
}
