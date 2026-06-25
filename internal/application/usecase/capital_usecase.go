package usecase

import (
	"context"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type CapitalInjectionService struct {
	Log  *zerolog.Logger
	DB   *gorm.DB
	Repo *repository.CapitalInjectionRepository
}

func NewCapitalInjectionService(ctx context.Context, db *gorm.DB) *CapitalInjectionService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &CapitalInjectionService{
		Log:  logger,
		DB:   db,
		Repo: repository.NewCapitalInjectionRepository(ctx, db),
	}
}

func (s *CapitalInjectionService) toResponse(r *entity.CapitalInjection) dto.CapitalInjectionResponse {
	t := r.Type
	if t == "" {
		t = "injection"
	}
	return dto.CapitalInjectionResponse{
		ID:         r.ID,
		Amount:     r.Amount,
		Type:       t,
		Source:     r.Source,
		Note:       r.Note,
		InjectedAt: r.InjectedAt.Format(time.RFC3339),
		CreatedBy:  r.CreatedBy,
		CreatedAt:  r.CreatedAt.Format(time.RFC3339),
	}
}

// parseInjectedAt accepts YYYY-MM-DD or RFC3339 string.
func parseInjectedAt(s string) (time.Time, error) {
	// Try date-only first (Bu Santi UX: input tanggal saja).
	if t, err := time.ParseInLocation("2006-01-02", s, time.Local); err == nil {
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

func (s *CapitalInjectionService) List(from, to string) ([]dto.CapitalInjectionResponse, *dto.ApiError) {
	var fromT, toT time.Time
	if from != "" {
		t, err := parseInjectedAt(from)
		if err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid from date"}
		}
		fromT = t
	}
	if to != "" {
		t, err := parseInjectedAt(to)
		if err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid to date"}
		}
		// End of day untuk inclusive
		toT = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
	}
	rows, err := s.Repo.FindByRange(fromT, toT)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch capital injections"}
	}
	out := make([]dto.CapitalInjectionResponse, 0, len(rows))
	for _, r := range rows {
		out = append(out, s.toResponse(&r))
	}
	return out, nil
}

func (s *CapitalInjectionService) Create(req dto.SaveCapitalInjectionRequest, userID *string) (*dto.CapitalInjectionResponse, *dto.ApiError) {
	injectedAt, err := parseInjectedAt(req.InjectedAt)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid injected_at format (YYYY-MM-DD)"}
	}
	t := req.Type
	if t == "" {
		t = "injection"
	}
	row := &entity.CapitalInjection{
		ID:         uuid.New().String(),
		Amount:     req.Amount,
		Type:       t,
		Source:     req.Source,
		Note:       req.Note,
		InjectedAt: injectedAt,
		CreatedBy:  userID,
	}
	if err := s.Repo.Create(row); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create capital injection")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create capital injection"}
	}
	resp := s.toResponse(row)
	return &resp, nil
}

func (s *CapitalInjectionService) Update(id string, req dto.SaveCapitalInjectionRequest) (*dto.CapitalInjectionResponse, *dto.ApiError) {
	row, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Capital injection not found"}
	}
	injectedAt, perr := parseInjectedAt(req.InjectedAt)
	if perr != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Invalid injected_at format"}
	}
	row.Amount = req.Amount
	if req.Type != "" {
		row.Type = req.Type
	}
	row.Source = req.Source
	row.Note = req.Note
	row.InjectedAt = injectedAt
	if err := s.Repo.Update(row); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update capital injection")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update"}
	}
	resp := s.toResponse(row)
	return &resp, nil
}

func (s *CapitalInjectionService) Delete(id string) *dto.ApiError {
	if _, err := s.Repo.FindByID(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Capital injection not found"}
	}
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete capital injection")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete"}
	}
	return nil
}
