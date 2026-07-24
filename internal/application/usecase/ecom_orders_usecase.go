package usecase

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomOrdersService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Cfg      *config.Config
}

func NewEcomOrdersService(ctx context.Context, db *gorm.DB) *EcomOrdersService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomOrdersService{DB: db, Log: logger, Cfg: cfg}
}

// midtransSnapURL — build hosted checkout URL dari snap_token + env.
// Format v3/redirection = Snap page terbaru (v2/vtweb deprecated).
// Sandbox vs Prod ditentukan dari MIDTRANS_IS_PROD (bukan dari isi token —
// token tidak punya prefix yang bisa di-detect).
// isStubToken — token yang di-generate saat Midtrans belum configured.
// FE tidak render tombol "Bayar Sekarang" untuk order dengan stub token.
func isStubToken(t string) bool {
	return len(t) >= 5 && t[:5] == "stub-"
}

func (s *EcomOrdersService) midtransSnapURL(token string) string {
	if token == "" {
		return ""
	}
	base := "https://app.sandbox.midtrans.com"
	if s.Cfg.MidtransIsProd {
		base = "https://app.midtrans.com"
	}
	return base + "/snap/v3/redirection/" + token
}

func (s *EcomOrdersService) List(userID string) ([]dto.CustomerOrderListItem, *dto.ApiError) {
	var orders []entity.Order
	if err := s.DB.Preload("Items").
		Where("ecom_user_id = ? AND deleted_at IS NULL", userID).
		Order("created_at DESC").
		Find(&orders).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch orders"}
	}
	out := make([]dto.CustomerOrderListItem, 0, len(orders))
	for _, o := range orders {
		ecomStatus := ""
		if o.EcomStatus != nil {
			ecomStatus = *o.EcomStatus
		}
		firstItem := ""
		if len(o.Items) > 0 {
			firstItem = o.Items[0].Name
		}
		row := dto.CustomerOrderListItem{
			ID:        o.ID,
			Total:     o.Total,
			EcomStatus: ecomStatus,
			ItemCount: len(o.Items),
			FirstItem: firstItem,
			CreatedAt: o.CreatedAt.Format(time.RFC3339),
		}
		if o.PaymentPaidAt != nil {
			s := o.PaymentPaidAt.Format(time.RFC3339)
			row.PaymentPaidAt = &s
		}
		out = append(out, row)
	}
	return out, nil
}

func (s *EcomOrdersService) GetDetail(userID, orderID string) (*dto.CustomerOrderDetail, *dto.ApiError) {
	var order entity.Order
	if err := s.DB.Preload("Items").
		Where("id = ? AND ecom_user_id = ? AND deleted_at IS NULL", orderID, userID).
		First(&order).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Order tidak ditemukan"}
	}

	ecomStatus := ""
	if order.EcomStatus != nil {
		ecomStatus = *order.EcomStatus
	}

	detail := &dto.CustomerOrderDetail{
		ID:           order.ID,
		Subtotal:     order.Subtotal,
		ShippingCost: order.ShippingCost,
		Total:        order.Total,
		EcomStatus:   ecomStatus,
		CreatedAt:    order.CreatedAt.Format(time.RFC3339),
	}

	// Items
	for _, it := range order.Items {
		detail.Items = append(detail.Items, dto.CustomerOrderItemDetail{
			ProductID: it.ProductID,
			Name:      it.Name,
			Quantity:  it.Quantity,
			UnitPrice: it.UnitPrice,
			Subtotal:  it.UnitPrice * float64(it.Quantity),
		})
	}

	// Shipping
	if order.ShippingCourier != nil {
		detail.Shipping.Courier = *order.ShippingCourier
	}
	if order.ShippingService != nil {
		detail.Shipping.ServiceName = *order.ShippingService
	}
	if order.ShippingETD != nil {
		detail.Shipping.ETD = *order.ShippingETD
	}
	if order.ShippingAWB != nil {
		detail.Shipping.AWB = *order.ShippingAWB
	}

	// Address snapshot (decode JSON)
	if order.ShippingAddressSnapshot != nil {
		var snap map[string]interface{}
		if err := json.Unmarshal([]byte(*order.ShippingAddressSnapshot), &snap); err == nil {
			get := func(k string) string {
				if v, ok := snap[k].(string); ok {
					return v
				}
				return ""
			}
			detail.Shipping.Address.Label = get("label")
			detail.Shipping.Address.RecipientName = get("recipient_name")
			detail.Shipping.Address.RecipientPhone = get("recipient_phone")
			detail.Shipping.Address.StreetAddress = get("street_address")
			detail.Shipping.Address.Subdistrict = get("subdistrict")
			detail.Shipping.Address.District = get("district")
			detail.Shipping.Address.City = get("city")
			detail.Shipping.Address.Province = get("province")
			detail.Shipping.Address.Zipcode = get("zipcode")
			detail.Shipping.Address.Notes = get("notes")
		}
	}

	// Payment — snap_token yang di-prefix "stub-" = stub dev mode, treat as
	// manual (BE belum di-config Midtrans / call failed saat checkout).
	if order.PaymentSnapToken != nil && *order.PaymentSnapToken != "" &&
		!isStubToken(*order.PaymentSnapToken) {
		detail.Payment.Mode = "midtrans"
		detail.Payment.SnapToken = *order.PaymentSnapToken
		detail.Payment.SnapRedirectURL = s.midtransSnapURL(*order.PaymentSnapToken)
	} else {
		detail.Payment.Mode = "manual"
	}
	if order.PaymentReference != nil {
		detail.Payment.Reference = *order.PaymentReference
	}
	if order.PaymentPaidAt != nil {
		s := order.PaymentPaidAt.Format(time.RFC3339)
		detail.Payment.PaidAt = &s
	}
	if order.PaymentExpiredAt != nil {
		s := order.PaymentExpiredAt.Format(time.RFC3339)
		detail.Payment.ExpiredAt = &s
	}

	return detail, nil
}
