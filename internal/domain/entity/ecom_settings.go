package entity

import "time"

// EcomSettings — singleton (id='default'). Sprint 4 Chunk 5 (31 Jul 2026).
// Terpisah dari POS `settings` supaya independent evolution.
type EcomSettings struct {
	ID string `gorm:"type:varchar(36);primary_key;default:'default'" json:"id"`

	MinOrderAmount float64 `gorm:"column:min_order_amount;type:decimal(15,2);not null;default:0" json:"min_order_amount"`

	WAContactNumber string  `gorm:"column:wa_contact_number;type:varchar(20);not null;default:''" json:"wa_contact_number"`
	WAPretext       *string `gorm:"column:wa_pretext;type:text;null" json:"wa_pretext,omitempty"`

	AnnouncementBarEnabled  bool    `gorm:"column:announcement_bar_enabled;type:tinyint(1);not null;default:0" json:"announcement_bar_enabled"`
	AnnouncementBarText     *string `gorm:"column:announcement_bar_text;type:varchar(200);null" json:"announcement_bar_text,omitempty"`
	AnnouncementBarCtaLabel *string `gorm:"column:announcement_bar_cta_label;type:varchar(50);null" json:"announcement_bar_cta_label,omitempty"`
	AnnouncementBarCtaURL   *string `gorm:"column:announcement_bar_cta_url;type:varchar(200);null" json:"announcement_bar_cta_url,omitempty"`

	StoreName          string  `gorm:"column:store_name;type:varchar(200);not null;default:'Toko Bahan Kue Santi'" json:"store_name"`
	StoreEmail         *string `gorm:"column:store_email;type:varchar(200);null" json:"store_email,omitempty"`
	StorePickupAddress *string `gorm:"column:store_pickup_address;type:text;null" json:"store_pickup_address,omitempty"`
	StorePickupPhone   *string `gorm:"column:store_pickup_phone;type:varchar(20);null" json:"store_pickup_phone,omitempty"`
	StorePickupAreaID  *string `gorm:"column:store_pickup_area_id;type:varchar(50);null" json:"store_pickup_area_id,omitempty"`

	PaymentPgEnabled     bool `gorm:"column:payment_pg_enabled;type:tinyint(1);not null;default:1" json:"payment_pg_enabled"`
	PaymentManualEnabled bool `gorm:"column:payment_manual_enabled;type:tinyint(1);not null;default:1" json:"payment_manual_enabled"`

	NotifOrderEmailEnabled bool `gorm:"column:notif_order_email_enabled;type:tinyint(1);not null;default:1" json:"notif_order_email_enabled"`

	CreatedAt time.Time `gorm:"default:current_timestamp()" json:"created_at,omitempty"`
	UpdatedAt time.Time `gorm:"default:current_timestamp()" json:"updated_at,omitempty"`
}

func (EcomSettings) TableName() string { return "ecom_settings" }
