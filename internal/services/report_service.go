package services

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/michaelbrian/kiosk/internal/models"
	"github.com/michaelbrian/kiosk/internal/repository"
)

type ReportService struct {
	saleRepo    *repository.SaleRepository
	productRepo *repository.ProductRepository
}

func NewReportService(saleRepo *repository.SaleRepository, productRepo *repository.ProductRepository) *ReportService {
	return &ReportService{saleRepo: saleRepo, productRepo: productRepo}
}

func (s *ReportService) GenerateReceiptPDF(sale *models.Sale) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A6", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.CellFormat(0, 10, "KIOSK MANAGER", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 6, fmt.Sprintf("Date: %s", sale.CreatedAt.Format("2006-01-02 15:04")), "", 1, "C", false, 0, "")
	receiptRef := sale.ID
	if len(receiptRef) > 8 {
		receiptRef = receiptRef[:8]
	}
	pdf.CellFormat(0, 6, fmt.Sprintf("Receipt: %s", receiptRef), "", 1, "C", false, 0, "")
	pdf.Ln(3)

	// Items table header
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(240, 240, 240)
	pdf.CellFormat(60, 7, "Item", "1", 0, "L", true, 0, "")
	pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 7, "Price", "1", 0, "R", true, 0, "")
	pdf.CellFormat(30, 7, "Subtotal", "1", 1, "R", true, 0, "")

	pdf.SetFont("Arial", "", 9)
	for _, item := range sale.Items {
		pdf.CellFormat(60, 6, item.ProductName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(15, 6, strconv.Itoa(item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 6, fmt.Sprintf("%.2f", item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 6, fmt.Sprintf("%.2f", item.Subtotal), "1", 1, "R", false, 0, "")
	}

	pdf.Ln(2)
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(100, 7, "Subtotal:", "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 7, fmt.Sprintf("KES %.2f", sale.TotalAmount), "", 1, "R", false, 0, "")

	if sale.DiscountAmount > 0 {
		pdf.CellFormat(100, 7, "Discount:", "", 0, "R", false, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("-KES %.2f", sale.DiscountAmount), "", 1, "R", false, 0, "")
	}

	pdf.SetFont("Arial", "B", 11)
	pdf.CellFormat(100, 8, "TOTAL:", "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, fmt.Sprintf("KES %.2f", sale.NetAmount), "", 1, "R", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.Ln(3)
	pdf.CellFormat(0, 6, fmt.Sprintf("Payment: %s", string(sale.PaymentMethod)), "", 1, "C", false, 0, "")
	if sale.CustomerName != "" {
		pdf.CellFormat(0, 6, fmt.Sprintf("Customer: %s", sale.CustomerName), "", 1, "C", false, 0, "")
	}
	pdf.Ln(2)
	pdf.CellFormat(0, 6, "Thank you for your purchase!", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (s *ReportService) ExportSalesCSV(ctx context.Context, from, to time.Time) ([]byte, error) {
	filter := repository.SaleFilter{From: from, To: to, Page: 1, Limit: 10000}
	sales, _, err := s.saleRepo.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"Sale ID", "Date", "Customer", "Total", "Discount", "Net Amount", "Payment Method", "Created By"})
	for _, s := range sales {
		_ = w.Write([]string{
			s.ID,
			s.CreatedAt.Format("2006-01-02 15:04:05"),
			s.CustomerName,
			fmt.Sprintf("%.2f", s.TotalAmount),
			fmt.Sprintf("%.2f", s.DiscountAmount),
			fmt.Sprintf("%.2f", s.NetAmount),
			string(s.PaymentMethod),
			s.CreatedByName,
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *ReportService) ExportInventoryCSV(ctx context.Context) ([]byte, error) {
	isActive := true
	products, _, err := s.productRepo.List(ctx, repository.ProductFilter{
		IsActive: &isActive,
		Page:     1,
		Limit:    100000,
	})
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"SKU", "Name", "Category", "Barcode", "Qty", "Reorder Level",
		"Buying Price", "Selling Price", "Stock Value", "Margin %", "Supplier"})
	for _, p := range products {
		_ = w.Write([]string{
			p.SKU,
			p.Name,
			p.CategoryName,
			p.Barcode,
			strconv.Itoa(p.Quantity),
			strconv.Itoa(p.ReorderLevel),
			fmt.Sprintf("%.2f", p.BuyingPrice),
			fmt.Sprintf("%.2f", p.SellingPrice),
			fmt.Sprintf("%.2f", p.BuyingPrice*float64(p.Quantity)),
			fmt.Sprintf("%.1f", p.Margin()),
			p.SupplierName,
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *ReportService) ExportPLCSV(ctx context.Context, from, to time.Time, groupBy string) ([]byte, error) {
	data, err := s.saleRepo.GetPLSummary(ctx, from, to, groupBy)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	_ = w.Write([]string{"Period", "Sales Count", "Revenue", "COGS", "Gross Profit", "Margin %"})
	for _, p := range data {
		_ = w.Write([]string{
			p.Period,
			strconv.Itoa(p.SaleCount),
			fmt.Sprintf("%.2f", p.Revenue),
			fmt.Sprintf("%.2f", p.COGS),
			fmt.Sprintf("%.2f", p.GrossProfit),
			fmt.Sprintf("%.1f", p.Margin),
		})
	}
	w.Flush()
	return buf.Bytes(), w.Error()
}

func (s *ReportService) GeneratePLPDF(ctx context.Context, from, to time.Time) ([]byte, error) {
	data, err := s.saleRepo.GetPLSummary(ctx, from, to, "day")
	if err != nil {
		return nil, err
	}

	var totalRev, totalCOGS, totalProfit float64
	for _, p := range data {
		totalRev += p.Revenue
		totalCOGS += p.COGS
		totalProfit += p.GrossProfit
	}

	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 18)
	pdf.CellFormat(0, 12, "Profit & Loss Report", "", 1, "C", false, 0, "")
	pdf.SetFont("Arial", "", 10)
	pdf.CellFormat(0, 7, fmt.Sprintf("Period: %s to %s", from.Format("2006-01-02"), to.Format("2006-01-02")), "", 1, "C", false, 0, "")
	pdf.Ln(5)

	// Summary
	pdf.SetFont("Arial", "B", 12)
	pdf.CellFormat(0, 8, "Summary", "", 1, "L", false, 0, "")
	pdf.SetFont("Arial", "", 11)
	pdf.CellFormat(60, 7, "Total Revenue:", "", 0, "L", false, 0, "")
	pdf.CellFormat(50, 7, fmt.Sprintf("KES %.2f", totalRev), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 7, "Total COGS:", "", 0, "L", false, 0, "")
	pdf.CellFormat(50, 7, fmt.Sprintf("KES %.2f", totalCOGS), "", 1, "L", false, 0, "")
	pdf.CellFormat(60, 7, "Gross Profit:", "", 0, "L", false, 0, "")
	pdf.CellFormat(50, 7, fmt.Sprintf("KES %.2f", totalProfit), "", 1, "L", false, 0, "")
	pdf.Ln(5)

	// Table
	pdf.SetFont("Arial", "B", 10)
	pdf.SetFillColor(66, 139, 202)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(50, 8, "Period", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 8, "Sales", "1", 0, "C", true, 0, "")
	pdf.CellFormat(55, 8, "Revenue", "1", 0, "C", true, 0, "")
	pdf.CellFormat(55, 8, "COGS", "1", 0, "C", true, 0, "")
	pdf.CellFormat(55, 8, "Gross Profit", "1", 0, "C", true, 0, "")
	pdf.CellFormat(30, 8, "Margin", "1", 1, "C", true, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(0, 0, 0)
	for i, p := range data {
		fill := i%2 == 0
		pdf.SetFillColor(245, 245, 245)
		pdf.CellFormat(50, 7, p.Period, "1", 0, "L", fill, 0, "")
		pdf.CellFormat(20, 7, strconv.Itoa(p.SaleCount), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(55, 7, fmt.Sprintf("%.2f", p.Revenue), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(55, 7, fmt.Sprintf("%.2f", p.COGS), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(55, 7, fmt.Sprintf("%.2f", p.GrossProfit), "1", 0, "R", fill, 0, "")
		pdf.CellFormat(30, 7, fmt.Sprintf("%.1f%%", p.Margin), "1", 1, "R", fill, 0, "")
	}

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// GetStockHistory wraps stock repo
type StockMovement = models.StockMovement

func (s *ReportService) GetStockHistory(ctx context.Context, productID string, page, limit int) ([]*models.StockMovement, int, error) {
	_ = ctx
	return nil, 0, nil // Placeholder - stock repo used in product service
}
