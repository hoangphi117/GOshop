package pdf

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jung-kurt/gofpdf"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

var (
	// Múi giờ Việt Nam (UTC+7 / ICT)
	vnTimezone = time.FixedZone("ICT", 7*3600)
)

type InvoiceItem struct {
	ProductName string
	Quantity    int
	UnitPrice   float64
	Subtotal    float64
}

type InvoiceData struct {
	OrderNumber     string
	OrderDate       time.Time
	CustomerName    string
	CustomerPhone   string
	CustomerAddress string
	PaymentMethod   string
	PaymentStatus   string
	TransactionRef  string
	PaidAt          *time.Time
	Items           []InvoiceItem
	SubtotalAmount  float64
	DiscountAmount  float64
	CouponCode      string
	TotalAmount     float64
}

func GenerateInvoicePDF(data InvoiceData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Chuyển đổi dữ liệu chữ tiếng Việt sang dạng không dấu chuẩn in ấn
	customerName := cleanVN(data.CustomerName)
	customerAddress := cleanVN(data.CustomerAddress)
	paymentMethod := cleanVN(data.PaymentMethod)
	paymentStatus := cleanVN(data.PaymentStatus)

	// Định dạng thời gian chuẩn múi giờ Việt Nam (+7)
	orderDateStr := formatVNTime(data.OrderDate, "15:04:05 02/01/2006")

	// 1. HEADER & LOGO BRANDING
	pdf.SetFillColor(0, 173, 216) // #00ADD8
	pdf.Rect(0, 0, 210, 8, "F")

	pdf.SetFont("Arial", "B", 22)
	pdf.SetTextColor(0, 173, 216)
	pdf.CellFormat(100, 10, "GOshop", "", 0, "L", false, 0, "")

	pdf.SetFont("Arial", "B", 14)
	pdf.SetTextColor(50, 50, 50)
	pdf.CellFormat(80, 10, "SALES INVOICE", "", 1, "R", false, 0, "")

	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(120, 120, 120)
	pdf.CellFormat(100, 5, "E-Commerce Platform | www.goshop.vn", "", 0, "L", false, 0, "")
	pdf.CellFormat(80, 5, fmt.Sprintf("Invoice #: %s", data.OrderNumber), "", 1, "R", false, 0, "")

	pdf.CellFormat(100, 5, "Hotline: 1900-xxxx | support@goshop.vn", "", 0, "L", false, 0, "")
	pdf.CellFormat(80, 5, fmt.Sprintf("Date: %s", orderDateStr), "", 1, "R", false, 0, "")
	pdf.Ln(8)

	// Đường kẻ phân cách
	pdf.SetDrawColor(220, 220, 220)
	pdf.Line(15, pdf.GetY(), 195, pdf.GetY())
	pdf.Ln(6)

	// 2. THÔNG TIN KHÁCH HÀNG & THANH TOÁN (2 Cột)
	yStart := pdf.GetY()

	// Cột trái: Khách hàng
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(30, 30, 30)
	pdf.Text(15, yStart, "CUSTOMER & SHIPPING INFO:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(70, 70, 70)
	pdf.Text(15, yStart+6, fmt.Sprintf("Recipient: %s", customerName))
	pdf.Text(15, yStart+11, fmt.Sprintf("Phone: %s", data.CustomerPhone))

	// Xuất địa chỉ tự động ngắt dòng
	pdf.SetXY(15, yStart+13)
	pdf.MultiCell(85, 4.5, fmt.Sprintf("Address: %s", customerAddress), "", "L", false)

	// Cột phải: Thanh toán
	pdf.SetFont("Arial", "B", 10)
	pdf.SetTextColor(30, 30, 30)
	pdf.Text(115, yStart, "PAYMENT DETAILS:")
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(70, 70, 70)
	pdf.Text(115, yStart+6, fmt.Sprintf("Method: %s", paymentMethod))
	pdf.Text(115, yStart+11, fmt.Sprintf("Status: %s", paymentStatus))
	if data.TransactionRef != "" {
		pdf.Text(115, yStart+16, fmt.Sprintf("Transaction Ref: %s", data.TransactionRef))
	}
	if data.PaidAt != nil && !data.PaidAt.IsZero() {
		paidDateStr := formatVNTime(*data.PaidAt, "15:04:05 02/01/2006")
		pdf.Text(115, yStart+21, fmt.Sprintf("Paid Date: %s", paidDateStr))
	}

	pdf.SetY(yStart + 28)

	// 3. BẢNG DANH SÁCH SẢN PHẨM
	pdf.SetFillColor(241, 248, 250)
	pdf.SetDrawColor(210, 230, 238)
	pdf.SetFont("Arial", "B", 9)
	pdf.SetTextColor(40, 40, 40)

	pdf.CellFormat(10, 8, "#", "1", 0, "C", true, 0, "")
	pdf.CellFormat(95, 8, "Product Description", "1", 0, "L", true, 0, "")
	pdf.CellFormat(20, 8, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(25, 8, "Unit Price", "1", 0, "R", true, 0, "")
	pdf.CellFormat(30, 8, "Subtotal", "1", 1, "R", true, 0, "")

	// Dòng sản phẩm
	pdf.SetFont("Arial", "", 9)
	pdf.SetTextColor(50, 50, 50)
	for i, item := range data.Items {
		cleanItemName := cleanVN(item.ProductName)
		pdf.CellFormat(10, 7, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(95, 7, cleanItemName, "1", 0, "L", false, 0, "")
		pdf.CellFormat(20, 7, fmt.Sprintf("%d", item.Quantity), "1", 0, "C", false, 0, "")
		pdf.CellFormat(25, 7, formatMoney(item.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(30, 7, formatMoney(item.Subtotal), "1", 1, "R", false, 0, "")
	}
	pdf.Ln(3)

	// 4. BẢNG TỔNG KẾT TIỀN
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(125, 6, "", "", 0, "", false, 0, "")
	pdf.CellFormat(25, 6, "Subtotal:", "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 6, formatMoney(data.SubtotalAmount), "", 1, "R", false, 0, "")

	if data.DiscountAmount > 0 {
		couponLabel := "Discount:"
		if data.CouponCode != "" {
			couponLabel = fmt.Sprintf("Discount (%s):", data.CouponCode)
		}
		pdf.SetTextColor(40, 167, 69) // Màu xanh lá
		pdf.CellFormat(125, 6, "", "", 0, "", false, 0, "")
		pdf.CellFormat(25, 6, couponLabel, "", 0, "R", false, 0, "")
		pdf.CellFormat(30, 6, fmt.Sprintf("-%s", formatMoney(data.DiscountAmount)), "", 1, "R", false, 0, "")
		pdf.SetTextColor(50, 50, 50)
	}

	pdf.CellFormat(125, 6, "", "", 0, "", false, 0, "")
	pdf.CellFormat(25, 6, "Shipping Fee:", "", 0, "R", false, 0, "")
	pdf.CellFormat(30, 6, "Free", "", 1, "R", false, 0, "")

	// Dòng Total Paid
	pdf.SetDrawColor(0, 173, 216)
	pdf.SetFont("Arial", "B", 11)
	pdf.SetTextColor(0, 173, 216)
	pdf.CellFormat(125, 8, "", "", 0, "", false, 0, "")
	pdf.CellFormat(25, 8, "Total Paid:", "T", 0, "R", false, 0, "")
	pdf.CellFormat(30, 8, formatMoney(data.TotalAmount), "T", 1, "R", false, 0, "")

	// 5. FOOTER & GHI CHÚ
	pdf.SetY(265)
	pdf.SetFont("Arial", "I", 8)
	pdf.SetTextColor(150, 150, 150)
	pdf.CellFormat(0, 4, "Thank you for choosing GOshop! This is an electronically generated sales invoice.", "", 1, "C", false, 0, "")
	pdf.CellFormat(0, 4, "For any inquiries, please contact our support team at support@goshop.vn.", "", 1, "C", false, 0, "")

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("failed to export pdf buffer: %w", err)
	}

	return buf.Bytes(), nil
}

func formatVNTime(t time.Time, layout string) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.In(vnTimezone).Format(layout)
}

func formatMoney(amount float64) string {
	integerPart := int64(amount)
	str := fmt.Sprintf("%d", integerPart)
	n := len(str)
	if n <= 3 {
		return fmt.Sprintf("%s VND", str)
	}

	var result []byte
	rem := n % 3
	if rem > 0 {
		result = append(result, str[:rem]...)
		if n > rem {
			result = append(result, ',')
		}
	}
	for i := rem; i < n; i += 3 {
		result = append(result, str[i:i+3]...)
		if i+3 < n {
			result = append(result, ',')
		}
	}
	return fmt.Sprintf("%s VND", string(result))
}

func cleanVN(s string) string {
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	result, _, _ := transform.String(t, s)
	result = strings.ReplaceAll(result, "đ", "d")
	result = strings.ReplaceAll(result, "Đ", "D")
	return result
}
