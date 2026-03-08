package notifications

// WhatsAppService is retained as a no-op stub so existing import paths compile.
// SMS notifications are now handled by SMSService (Celcom Africa).
type WhatsAppService struct{}

func NewWhatsAppService(_ interface{}) *WhatsAppService { return &WhatsAppService{} }

func (s *WhatsAppService) Send(_, _ string) error                             { return nil }
func (s *WhatsAppService) SendLowStockAlert(_ string, _ []LowStockItem) error { return nil }
func (s *WhatsAppService) SendDailySummary(_ string, _ DailySummary) error    { return nil }
