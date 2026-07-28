package dto

type AddressResponse struct {
	ID             string   `json:"id"`
	Label          string   `json:"label"`
	RecipientName  string   `json:"recipient_name"`
	RecipientPhone string   `json:"recipient_phone"`
	Province       string   `json:"province"`
	City           string   `json:"city"`
	District       string   `json:"district"`
	Subdistrict    string   `json:"subdistrict"`
	Zipcode        string   `json:"zipcode"`
	// Biteship area_id (dari Maps API). Optional untuk address lama — FE
	// bisa render badge "Alamat Belum Presisi" + tombol re-resolve.
	BiteshipAreaID *string  `json:"biteship_area_id,omitempty"`
	StreetAddress  string   `json:"street_address"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
	IsDefault      bool     `json:"is_default"`
}

type AddressCreateRequest struct {
	Label          string   `json:"label" validate:"required,min=1,max=50"`
	RecipientName  string   `json:"recipient_name" validate:"required,min=2,max=200"`
	RecipientPhone string   `json:"recipient_phone" validate:"required,min=8,max=20"`
	Province       string   `json:"province" validate:"required"`
	City           string   `json:"city" validate:"required"`
	District       string   `json:"district" validate:"required"`
	Subdistrict    string   `json:"subdistrict" validate:"required"`
	Zipcode        string   `json:"zipcode" validate:"required,min=5,max=10"`
	// Optional untuk backward compat address form lama. FE combobox baru
	// wajib set — di-resolve dari Maps API saat customer pilih kelurahan.
	BiteshipAreaID *string  `json:"biteship_area_id,omitempty"`
	StreetAddress  string   `json:"street_address" validate:"required,min=5"`
	Latitude       *float64 `json:"latitude,omitempty"`
	Longitude      *float64 `json:"longitude,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
	IsDefault      bool     `json:"is_default"`
}

// AddressUpdateRequest — sama structure, semua field bisa di-update.
type AddressUpdateRequest = AddressCreateRequest
