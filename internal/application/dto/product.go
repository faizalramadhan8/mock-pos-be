package dto

type CreateProductRequest struct {
	SKU           string  `json:"sku" validate:"required"`
	Barcode       string  `json:"barcode"`
	Name          string  `json:"name" validate:"required"`
	NameID        string  `json:"name_id"`
	CategoryID    string  `json:"category_id" validate:"required"`
	PurchasePrice float64 `json:"purchase_price" validate:"min=0"`
	SellingPrice  float64 `json:"selling_price" validate:"min=0"`
	QtyPerBox     int     `json:"qty_per_box"`
	Stock         int     `json:"stock"`
	Unit          string  `json:"unit" validate:"required"`
	Image         string  `json:"image"`
	MinStock      int     `json:"min_stock"`
}

type UpdateProductRequest struct {
	Name          string  `json:"name" validate:"omitempty"`
	NameID        string  `json:"name_id"`
	CategoryID    string  `json:"category_id"`
	PurchasePrice float64 `json:"purchase_price"`
	SellingPrice  float64 `json:"selling_price"`
	QtyPerBox     int     `json:"qty_per_box"`
	Stock         *int    `json:"stock"`
	Unit          string  `json:"unit"`
	Image         string  `json:"image"`
	MinStock      int     `json:"min_stock"`
	SKU           string  `json:"sku"`
	Barcode       string  `json:"barcode"`
}

type AdjustStockRequest struct {
	Delta int `json:"delta" validate:"required"`
}

type ProductResponse struct {
	ID            string           `json:"id"`
	SKU           string           `json:"sku"`
	Barcode       string           `json:"barcode,omitempty"`
	Name          string           `json:"name"`
	NameID        string           `json:"name_id"`
	CategoryID    string           `json:"category_id"`
	Category      *CategoryResponse `json:"category,omitempty"`
	PurchasePrice float64          `json:"purchase_price"`
	SellingPrice  float64          `json:"selling_price"`
	QtyPerBox     int              `json:"qty_per_box"`
	Stock         int              `json:"stock"`
	Unit          string           `json:"unit"`
	Image         string           `json:"image,omitempty"`
	MinStock      int              `json:"min_stock"`
	IsActive      bool             `json:"is_active"`
	CreatedAt     string           `json:"created_at"`
}

type CreateCategoryRequest struct {
	Name   string `json:"name" validate:"required"`
	NameID string `json:"name_id"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
}

type UpdateCategoryRequest struct {
	Name   string `json:"name"`
	NameID string `json:"name_id"`
	Icon   string `json:"icon"`
	Color  string `json:"color"`
}

type CategoryResponse struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	NameID string `json:"name_id"`
	Icon   string `json:"icon,omitempty"`
	Color  string `json:"color,omitempty"`
}

type CreateSupplierRequest struct {
	Name    string `json:"name" validate:"required"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

type UpdateSupplierRequest struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Email   string `json:"email"`
	Address string `json:"address"`
}

type SupplierResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Phone     string `json:"phone,omitempty"`
	Email     string `json:"email,omitempty"`
	Address   string `json:"address,omitempty"`
	CreatedAt string `json:"created_at"`
}
