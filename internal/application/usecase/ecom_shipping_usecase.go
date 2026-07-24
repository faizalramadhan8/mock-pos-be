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
		DB:  db,
		Log: logger,
		Biteship: biteship.NewClient(
			cfg.BiteshipAPIKey,
			cfg.BiteshipOriginArea,
			cfg.BiteshipOriginPostal,
			cfg.BiteshipCouriers,
			cfg.ShippingFlatBaseRate,
		),
	}
}

// GetRates — kalkulasi shipping option berdasar alamat + cart total berat.
func (s *EcomShippingService) GetRates(userID string, req dto.ShippingRateRequest) (*dto.ShippingRatesResponse, *dto.ApiError) {
	// Load address
	var addr entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", req.AddressID, userID).First(&addr).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	// Cart → line items untuk Biteship (matches Postman items shape).
	var cart []entity.EcomCartItem
	if err := s.DB.Preload("Product").Where("user_id = ?", userID).Find(&cart).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch cart"}
	}
	items := make([]biteship.Item, 0, len(cart))
	totalWeight := 0
	for _, c := range cart {
		if c.Product == nil {
			continue
		}
		// Berat per unit — fallback 200g kalau admin belum set.
		w := 200
		if c.Product.EcomWeightGrams != nil && *c.Product.EcomWeightGrams > 0 {
			w = *c.Product.EcomWeightGrams
		}
		// Harga per unit untuk value (dipakai insurance calc kurir).
		value := int(c.Product.SellingPrice)
		if c.Product.EcomPrice != nil {
			value = int(*c.Product.EcomPrice)
		}
		items = append(items, biteship.Item{
			Name:     c.Product.Name,
			Value:    value,
			Weight:   w,
			Quantity: c.Quantity,
		})
		totalWeight += w * c.Quantity
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
