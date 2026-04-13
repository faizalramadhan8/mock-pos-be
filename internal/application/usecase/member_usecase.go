package usecase

import (
	"context"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type MemberService struct {
	Log       *zerolog.Logger
	Repo      *repository.MemberRepository
	OrderRepo *repository.OrderRepository
}

func NewMemberService(ctx context.Context, db *gorm.DB) *MemberService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &MemberService{
		Log:       logger,
		Repo:      repository.NewMemberRepository(ctx, db),
		OrderRepo: repository.NewOrderRepository(ctx, db),
	}
}

func (s *MemberService) GetAll(search string, page, limit int) ([]dto.MemberResponse, int64, *dto.ApiError) {
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	offset := (page - 1) * limit

	members, total, err := s.Repo.FindAll(search, limit, offset)
	if err != nil {
		return nil, 0, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch members"}
	}

	var result []dto.MemberResponse
	for _, m := range members {
		result = append(result, dto.MemberResponse{
			ID:        m.ID,
			Name:      m.Name,
			Phone:     m.Phone,
			CreatedAt: m.CreatedAt.Format(time.RFC3339),
		})
	}
	return result, total, nil
}

func (s *MemberService) Create(req dto.CreateMemberRequest) (*dto.MemberResponse, *dto.ApiError) {
	member := &entity.Member{
		ID:    uuid.New().String(),
		Name:  req.Name,
		Phone: req.Phone,
	}

	if err := s.Repo.Create(member); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create member")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create member"}
	}

	return &dto.MemberResponse{
		ID:        member.ID,
		Name:      member.Name,
		Phone:     member.Phone,
		CreatedAt: member.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *MemberService) SearchByPhone(phone string) (*dto.MemberResponse, *dto.ApiError) {
	member, err := s.Repo.FindByPhone(phone)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Member not found"}
	}
	return &dto.MemberResponse{
		ID:        member.ID,
		Name:      member.Name,
		Phone:     member.Phone,
		CreatedAt: member.CreatedAt.Format(time.RFC3339),
	}, nil
}

func (s *MemberService) Delete(id string) *dto.ApiError {
	if err := s.Repo.Delete(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to delete member"}
	}
	return nil
}

// GetStats aggregates a member's purchase activity over an optional date range.
// from/to are inclusive YYYY-MM-DD strings. Empty string means unbounded.
// Lifetime totals are always computed across all completed orders.
func (s *MemberService) GetStats(memberID, from, to string) (*dto.MemberStatsResponse, *dto.ApiError) {
	if _, err := s.Repo.FindByID(memberID); err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Member not found"}
	}

	rangeOrders, _, err := s.OrderRepo.FindByMember(memberID, from, to, 0, 0)
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch member range orders")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch stats"}
	}

	lifetimeOrders, _, err := s.OrderRepo.FindByMember(memberID, "", "", 0, 0)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to fetch lifetime stats"}
	}

	resp := &dto.MemberStatsResponse{
		MemberID: memberID,
		From:     from,
		To:       to,
	}

	// Range stats
	var lastVisit time.Time
	monthlyAgg := map[string]*dto.MemberMonthlyBreakdown{}
	productAgg := map[string]*dto.MemberTopProduct{}

	for _, o := range rangeOrders {
		resp.TotalSpend += o.Total
		resp.OrderCount++
		if o.CreatedAt.After(lastVisit) {
			lastVisit = o.CreatedAt
		}

		monthKey := o.CreatedAt.Format("2006-01")
		mb, ok := monthlyAgg[monthKey]
		if !ok {
			mb = &dto.MemberMonthlyBreakdown{Month: monthKey}
			monthlyAgg[monthKey] = mb
		}
		mb.Spend += o.Total
		mb.Orders++

		for _, item := range o.Items {
			if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
				saved := (*item.RegularPrice - item.UnitPrice) * float64(item.Quantity)
				resp.TotalSavings += saved
				mb.Savings += saved
			}
			tp, ok := productAgg[item.ProductID]
			if !ok {
				tp = &dto.MemberTopProduct{ProductID: item.ProductID, Name: item.Name}
				productAgg[item.ProductID] = tp
			}
			tp.Quantity += item.Quantity
			tp.Spend += item.UnitPrice * float64(item.Quantity)
		}
	}

	if resp.OrderCount > 0 {
		resp.AvgBasket = resp.TotalSpend / float64(resp.OrderCount)
	}
	if !lastVisit.IsZero() {
		resp.LastVisit = lastVisit.Format(time.RFC3339)
	}

	// Lifetime totals
	for _, o := range lifetimeOrders {
		resp.LifetimeSpend += o.Total
		resp.LifetimeOrders++
	}

	// Monthly breakdown sorted ascending
	months := make([]string, 0, len(monthlyAgg))
	for k := range monthlyAgg {
		months = append(months, k)
	}
	sortStrings(months)
	for _, k := range months {
		resp.MonthlyBreakdown = append(resp.MonthlyBreakdown, *monthlyAgg[k])
	}

	// Top 5 products by spend
	tops := make([]dto.MemberTopProduct, 0, len(productAgg))
	for _, p := range productAgg {
		tops = append(tops, *p)
	}
	sortTopProducts(tops)
	if len(tops) > 5 {
		tops = tops[:5]
	}
	resp.TopProducts = tops

	return resp, nil
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func sortTopProducts(s []dto.MemberTopProduct) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1].Spend < s[j].Spend; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
