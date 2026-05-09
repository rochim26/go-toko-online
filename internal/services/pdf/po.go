package pdf

import (
	"fmt"

	"github.com/jung-kurt/gofpdf"
	"github.com/tokoonline/app/internal/models"
)

type POData struct {
	StoreName     string
	StoreAddress  string
	StoreNPWP     string
	BuyerName     string
	BuyerAddress  string
	BuyerNPWP     string
	PONumber      string
	IssuedAt      string
	DueAt         string
	Order         *models.Order
}

func GeneratePO(d POData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.AddPage()

	// Header
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "PURCHASE ORDER")
	pdf.Ln(12)

	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(95, 6, d.StoreName)
	pdf.Cell(0, 6, "PO No: "+d.PONumber)
	pdf.Ln(6)

	pdf.SetFont("Arial", "", 9)
	pdf.MultiCell(95, 5, d.StoreAddress, "", "L", false)
	yA := pdf.GetY()
	pdf.SetXY(110, yA-10)
	pdf.Cell(0, 5, "Tanggal: "+d.IssuedAt)
	pdf.Ln(5)
	pdf.SetX(110)
	if d.DueAt != "" {
		pdf.Cell(0, 5, "Jatuh Tempo: "+d.DueAt)
		pdf.Ln(5)
	}
	if d.StoreNPWP != "" {
		pdf.SetX(110)
		pdf.Cell(0, 5, "NPWP: "+d.StoreNPWP)
		pdf.Ln(5)
	}

	pdf.Ln(4)
	pdf.SetFont("Arial", "B", 10)
	pdf.Cell(0, 6, "Tagih kepada:")
	pdf.Ln(6)
	pdf.SetFont("Arial", "", 9)
	pdf.Cell(0, 5, d.BuyerName)
	pdf.Ln(5)
	if d.BuyerAddress != "" {
		pdf.MultiCell(0, 5, d.BuyerAddress, "", "L", false)
	}
	if d.BuyerNPWP != "" {
		pdf.Cell(0, 5, "NPWP: "+d.BuyerNPWP)
		pdf.Ln(5)
	}

	pdf.Ln(4)
	// Items table
	pdf.SetFont("Arial", "B", 9)
	pdf.SetFillColor(230, 230, 230)
	pdf.CellFormat(15, 7, "No", "1", 0, "C", true, 0, "")
	pdf.CellFormat(40, 7, "SKU", "1", 0, "C", true, 0, "")
	pdf.CellFormat(75, 7, "Nama Produk", "1", 0, "C", true, 0, "")
	pdf.CellFormat(15, 7, "Qty", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 7, "Harga", "1", 0, "C", true, 0, "")
	pdf.CellFormat(20, 7, "Total", "1", 0, "C", true, 0, "")
	pdf.Ln(7)

	pdf.SetFont("Arial", "", 8)
	for i, it := range d.Order.Items {
		pdf.CellFormat(15, 6, fmt.Sprintf("%d", i+1), "1", 0, "C", false, 0, "")
		pdf.CellFormat(40, 6, it.SKU, "1", 0, "L", false, 0, "")
		pdf.CellFormat(75, 6, truncate(it.Name, 50), "1", 0, "L", false, 0, "")
		pdf.CellFormat(15, 6, fmt.Sprintf("%d", it.Qty), "1", 0, "C", false, 0, "")
		pdf.CellFormat(20, 6, fmtIDR(it.UnitPrice), "1", 0, "R", false, 0, "")
		pdf.CellFormat(20, 6, fmtIDR(it.LineTotal), "1", 0, "R", false, 0, "")
		pdf.Ln(6)
	}

	// Totals
	pdf.Ln(2)
	pdf.SetFont("Arial", "", 9)
	pdf.CellFormat(145, 6, "Subtotal", "0", 0, "R", false, 0, "")
	pdf.CellFormat(40, 6, fmtIDR(d.Order.Subtotal), "0", 0, "R", false, 0, "")
	pdf.Ln(6)
	if d.Order.DiscountTotal > 0 {
		pdf.CellFormat(145, 6, "Diskon", "0", 0, "R", false, 0, "")
		pdf.CellFormat(40, 6, "-"+fmtIDR(d.Order.DiscountTotal), "0", 0, "R", false, 0, "")
		pdf.Ln(6)
	}
	if d.Order.ShippingTotal > 0 {
		pdf.CellFormat(145, 6, "Ongkos Kirim", "0", 0, "R", false, 0, "")
		pdf.CellFormat(40, 6, fmtIDR(d.Order.ShippingTotal), "0", 0, "R", false, 0, "")
		pdf.Ln(6)
	}
	if d.Order.TaxTotal > 0 {
		pdf.CellFormat(145, 6, "PPN", "0", 0, "R", false, 0, "")
		pdf.CellFormat(40, 6, fmtIDR(d.Order.TaxTotal), "0", 0, "R", false, 0, "")
		pdf.Ln(6)
	}
	pdf.SetFont("Arial", "B", 10)
	pdf.CellFormat(145, 7, "GRAND TOTAL", "T", 0, "R", false, 0, "")
	pdf.CellFormat(40, 7, fmtIDR(d.Order.GrandTotal), "T", 0, "R", false, 0, "")
	pdf.Ln(10)

	pdf.SetFont("Arial", "I", 8)
	pdf.MultiCell(0, 4, "Dokumen ini diterbitkan secara elektronik dan sah tanpa tanda tangan basah.", "", "L", false)

	var buf nullWriter
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.b, nil
}

type nullWriter struct{ b []byte }

func (w *nullWriter) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func fmtIDR(v float64) string {
	// Simple IDR formatter: "Rp 1.234.567"
	negative := v < 0
	if negative {
		v = -v
	}
	intPart := int64(v + 0.5)
	s := fmt.Sprintf("%d", intPart)
	// insert thousands separator
	out := []byte{}
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	res := "Rp " + string(out)
	if negative {
		res = "-" + res
	}
	return res
}
