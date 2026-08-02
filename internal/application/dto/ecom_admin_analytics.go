package dto

// Sprint 5 Chunk 8 (2 Aug 2026) — Analytics DTOs.

type EcomAdminAnalytics struct {
	RangeFrom       string                   `json:"range_from"`
	RangeTo         string                   `json:"range_to"`
	TotalOrders     int                      `json:"total_orders"`
	CompletedCount  int                      `json:"completed_count"`
	TotalRevenue    float64                  `json:"total_revenue"`
	AvgOrderValue   float64                  `json:"avg_order_value"`
	ConversionRate  float64                  `json:"conversion_rate"` // 0-100
	CancelRate      float64                  `json:"cancel_rate"`     // 0-100
	DailyRevenue    []AnalyticsDailyBucket   `json:"daily_revenue"`
	Funnel          map[string]int           `json:"funnel"`
	PaymentChannels []AnalyticsChannelSlice  `json:"payment_channels"`
}

type AnalyticsDailyBucket struct {
	Date    string  `json:"date"` // YYYY-MM-DD
	Revenue float64 `json:"revenue"`
	Orders  int     `json:"orders"`
}

type AnalyticsChannelSlice struct {
	Method string  `json:"method"`
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}
