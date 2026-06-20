package dto

type RedeemableItemResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	PointsCost  int    `json:"points_cost"`
	Stock       int    `json:"stock"`
	Redeemed    int    `json:"redeemed"`
	IsActive    bool   `json:"is_active"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

type SaveRedeemableItemRequest struct {
	Name        string `json:"name" validate:"required,min=1"`
	Description string `json:"description"`
	Image       string `json:"image"`
	PointsCost  int    `json:"points_cost" validate:"required,min=1"`
	Stock       int    `json:"stock" validate:"min=0"`
	IsActive    *bool  `json:"is_active"`
}
