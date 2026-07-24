package usecase

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type EcomAddressService struct {
	DB  *gorm.DB
	Log *zerolog.Logger
}

func NewEcomAddressService(ctx context.Context, db *gorm.DB) *EcomAddressService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &EcomAddressService{DB: db, Log: logger}
}

func (s *EcomAddressService) List(userID string) ([]dto.AddressResponse, *dto.ApiError) {
	var rows []entity.EcomAddress
	if err := s.DB.Where("user_id = ? AND deleted_at IS NULL", userID).
		Order("is_default DESC, created_at DESC").Find(&rows).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch addresses"}
	}
	out := make([]dto.AddressResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, toAddressResponse(&r))
	}
	return out, nil
}

func (s *EcomAddressService) Create(userID string, req dto.AddressCreateRequest) (*dto.AddressResponse, *dto.ApiError) {
	// Kalau ini address pertama, force is_default=true (customer harus punya
	// default supaya fast-checkout jalan).
	var count int64
	s.DB.Model(&entity.EcomAddress{}).Where("user_id = ? AND deleted_at IS NULL", userID).Count(&count)
	isDefault := req.IsDefault || count == 0

	// Kalau set jadi default, unset default lain dulu (1 per user rule).
	if isDefault {
		s.DB.Model(&entity.EcomAddress{}).
			Where("user_id = ? AND deleted_at IS NULL AND is_default = 1", userID).
			Update("is_default", false)
	}

	row := entity.EcomAddress{
		ID:             uuid.New().String(),
		UserID:         userID,
		Label:          req.Label,
		RecipientName:  req.RecipientName,
		RecipientPhone: req.RecipientPhone,
		Province:       req.Province,
		City:           req.City,
		District:       req.District,
		Subdistrict:    req.Subdistrict,
		Zipcode:        req.Zipcode,
		StreetAddress:  req.StreetAddress,
		Latitude:       req.Latitude,
		Longitude:      req.Longitude,
		Notes:          req.Notes,
		IsDefault:      isDefault,
	}
	if err := s.DB.Create(&row).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save address"}
	}
	resp := toAddressResponse(&row)
	return &resp, nil
}

func (s *EcomAddressService) Update(userID, id string, req dto.AddressUpdateRequest) (*dto.AddressResponse, *dto.ApiError) {
	var row entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).First(&row).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}

	if req.IsDefault && !row.IsDefault {
		// Unset default lain (1 per user).
		s.DB.Model(&entity.EcomAddress{}).
			Where("user_id = ? AND deleted_at IS NULL AND is_default = 1", userID).
			Update("is_default", false)
	}

	row.Label = req.Label
	row.RecipientName = req.RecipientName
	row.RecipientPhone = req.RecipientPhone
	row.Province = req.Province
	row.City = req.City
	row.District = req.District
	row.Subdistrict = req.Subdistrict
	row.Zipcode = req.Zipcode
	row.StreetAddress = req.StreetAddress
	row.Latitude = req.Latitude
	row.Longitude = req.Longitude
	row.Notes = req.Notes
	row.IsDefault = req.IsDefault

	if err := s.DB.Save(&row).Error; err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update"}
	}
	resp := toAddressResponse(&row)
	return &resp, nil
}

func (s *EcomAddressService) Delete(userID, id string) *dto.ApiError {
	var row entity.EcomAddress
	if err := s.DB.Where("id = ? AND user_id = ? AND deleted_at IS NULL", id, userID).First(&row).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Alamat tidak ditemukan"}
	}
	if err := s.DB.Delete(&row).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete"}
	}
	// Kalau yang dihapus adalah default, promote address lain jadi default.
	if row.IsDefault {
		var next entity.EcomAddress
		if err := s.DB.Where("user_id = ? AND deleted_at IS NULL", userID).
			Order("created_at DESC").First(&next).Error; err == nil {
			next.IsDefault = true
			s.DB.Save(&next)
		}
	}
	return nil
}

func toAddressResponse(a *entity.EcomAddress) dto.AddressResponse {
	return dto.AddressResponse{
		ID:             a.ID,
		Label:          a.Label,
		RecipientName:  a.RecipientName,
		RecipientPhone: a.RecipientPhone,
		Province:       a.Province,
		City:           a.City,
		District:       a.District,
		Subdistrict:    a.Subdistrict,
		Zipcode:        a.Zipcode,
		StreetAddress:  a.StreetAddress,
		Latitude:       a.Latitude,
		Longitude:      a.Longitude,
		Notes:          a.Notes,
		IsDefault:      a.IsDefault,
	}
}
