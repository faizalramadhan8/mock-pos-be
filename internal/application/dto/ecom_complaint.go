package dto

// ComplaintSubmitRequest — customer submit komplain baru untuk order.
type ComplaintSubmitRequest struct {
	OrderID     string   `json:"order_id" validate:"required,uuid"`
	Reason      string   `json:"reason" validate:"required,oneof=barang_rusak barang_salah barang_kurang lainnya"`
	Description string   `json:"description" validate:"required,min=10,max=1000"`
	Images      []string `json:"images,omitempty" validate:"max=5,dive,url"`
}

// ComplaintResponse — untuk display customer + admin.
type ComplaintResponse struct {
	ID          string   `json:"id"`
	OrderID     string   `json:"order_id"`
	UserID      string   `json:"user_id"`
	UserName    string   `json:"user_name,omitempty"`   // Admin view — join dari users
	Reason      string   `json:"reason"`
	ReasonLabel string   `json:"reason_label"`          // Indonesian friendly label
	Description string   `json:"description"`
	Images      []string `json:"images"`
	Status      string   `json:"status"`
	AdminReply  string   `json:"admin_reply,omitempty"`
	CreatedAt   string   `json:"created_at"`
	ResolvedAt  string   `json:"resolved_at,omitempty"`
}

// ComplaintAdminReplyRequest — admin reply + set status.
type ComplaintAdminReplyRequest struct {
	Reply  string `json:"reply" validate:"required,min=5,max=1000"`
	Status string `json:"status" validate:"required,oneof=in_review resolved rejected"`
}
