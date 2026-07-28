package usecase

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/biteship"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomShippingService struct {
	DB       *gorm.DB
	Log      *zerolog.Logger
	Biteship *biteship.Client
}

func NewEcomShippingService(ctx context.Context, db *gorm.DB) *EcomShippingService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	cfg := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	return &EcomShippingService{
		DB:       db,
		Log:      logger,
		Biteship: NewBiteshipClient(cfg),
	}
}

// NewBiteshipClient — shared factory supaya multiple service pakai config
// yang sama (shipping rate + admin orders create + webhook verify).
func NewBiteshipClient(cfg *config.Config) *biteship.Client {
	return biteship.NewClient(
		cfg.BiteshipAPIKey,
		cfg.BiteshipOriginArea,
		cfg.BiteshipOriginPostal,
		cfg.BiteshipCouriers,
		cfg.ShippingFlatBaseRate,
	).WithShipper(
		cfg.BiteshipOriginName,
		cfg.BiteshipOriginPhone,
		cfg.BiteshipOriginAddress,
		cfg.BiteshipOriginNote,
		cfg.BiteshipWebhookSecret,
	)
}

// GetRates — kalkulasi shipping option berdasar alamat + cart total berat.
func (s *EcomShippingService) GetRates(userID string, req dto.ShippingRateRequest) (*dto.ShippingRatesResponse, *dto.ApiError) {
	// Load address
	var addr entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.AddressID, userID).First(&addr).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	// Items untuk Biteship — bisa dari 3 source (sama pattern seperti CreateOrder):
	//   1. BuyNowItems (bypass cart, dari PDP)
	//   2. SelectedItemIDs (partial cart)
	//   3. Default: seluruh cart
	items := make([]biteship.Item, 0)
	totalWeight := 0

	appendFromProduct := func(p *entity.Product, qty int) {
		if p == nil {
			return
		}
		w := 200
		if p.EcomWeightGrams != nil && *p.EcomWeightGrams > 0 {
			w = *p.EcomWeightGrams
		}
		value := int(p.SellingPrice)
		if p.EcomPrice != nil {
			value = int(*p.EcomPrice)
		}
		items = append(items, biteship.Item{Name: p.Name, Value: value, Weight: w, Quantity: qty})
		totalWeight += w * qty
	}

	if len(req.BuyNowItems) > 0 {
		for _, bn := range req.BuyNowItems {
			var p entity.Product
			if err := s.DB.Where("id = ? AND deleted_at IS NULL", bn.ProductID).First(&p).Error; err == nil {
				appendFromProduct(&p, bn.Quantity)
			}
		}
	} else {
		var cart []entity.EcomCartItem
		q := s.DB.Preload("Product").Where("user_id = ?", userID)
		if len(req.SelectedItemIDs) > 0 {
			q = q.Where("id IN ?", req.SelectedItemIDs)
		}
		if err := q.Find(&cart).Error; err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
		}
		for _, c := range cart {
			appendFromProduct(c.Product, c.Quantity)
		}
	}
	if totalWeight == 0 {
		totalWeight = 1000
	}

	// Biteship — coba area_id, postal_code, lat/lng berurutan sesuai ketersediaan.
	rates, err := s.Biteship.GetRates(biteship.RateRequest{
		DestinationPostal: addr.Zipcode,
		DestinationLat:    addr.Latitude,
		DestinationLng:    addr.Longitude,
		Items:             items,
		TotalWeightGrams:  totalWeight,
	})
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to get shipping rates")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Gagal ambil ongkir"}
	}

	resp := &dto.ShippingRatesResponse{TotalWeightGrams: totalWeight}
	resp.Address.Label = addr.Label
	resp.Address.RecipientName = addr.RecipientName
	resp.Address.City = addr.City
	resp.Address.Province = addr.Province
	for _, r := range rates {
		resp.Rates = append(resp.Rates, dto.ShippingRate{
			Courier: r.Courier, CourierName: r.CourierName,
			Service: r.Service, ServiceName: r.ServiceName,
			Cost: r.Cost, ETD: r.ETD,
		})
	}
	return resp, nil
}
