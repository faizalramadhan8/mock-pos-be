package usecase

import (
	"context"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type DashboardService struct {
	Log          *zerolog.Logger
	OrderRepo    *repository.OrderRepository
	ProductRepo  *repository.ProductRepository
	BatchRepo    *repository.StockBatchRepository
}

func NewDashboardService(ctx context.Context, db *gorm.DB) *DashboardService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	return &DashboardService{
		Log:         logger,
		OrderRepo:   repository.NewOrderRepository(ctx, db),
		ProductRepo: repository.NewProductRepository(ctx, db),
		BatchRepo:   repository.NewStockBatchRepository(ctx, db),
	}
}

func (s *DashboardService) GetDashboard(role string) (*dto.DashboardResponse, *dto.ApiError) {
	resp := &dto.DashboardResponse{}

	// Revenue and order count (for owner/cashier)
	revenue, orderCount, err := s.OrderRepo.GetRevenueStats()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to get revenue stats")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to get dashboard data"}
	}
	resp.Revenue = revenue
	resp.OrderCount = orderCount

	// Product count
	productCount, _ := s.ProductRepo.CountActive()
	resp.ProductCount = productCount

	// Low stock
	lowStockProducts, _ := s.ProductRepo.FindLowStock()
	resp.LowStockCount = int64(len(lowStockProducts))

	// Low stock items (up to 5)
	productService := &ProductService{Log: s.Log, Repo: s.ProductRepo}
	for i, p := range lowStockProducts {
		if i >= 5 {
			break
		}
		resp.LowStockItems = append(resp.LowStockItems, productService.toResponse(&p))
	}

	// Recent orders (up to 4)
	orders, _, _ := s.OrderRepo.FindAll("", 4, 0)
	orderService := &OrderService{Log: s.Log, Repo: s.OrderRepo}
	for _, o := range orders {
		resp.RecentOrders = append(resp.RecentOrders, orderService.toResponse(&o))
	}

	// Expiring batches (for staff role)
	if role == "staff" || role == "admin" || role == "superadmin" {
		expiringBatches, _ := s.BatchRepo.FindExpiring(60)
		invService := &InventoryService{Log: s.Log, BatchRepo: s.BatchRepo}
		for _, b := range expiringBatches {
			resp.ExpiringBatches = append(resp.ExpiringBatches, invService.toBatchResponse(&b))
		}
	}

	return resp, nil
}
