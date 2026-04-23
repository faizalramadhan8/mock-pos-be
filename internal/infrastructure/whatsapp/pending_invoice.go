package whatsapp

import (
	"fmt"
	"strings"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
)

// FormatPendingInvoice renders a WA message for a pending (unpaid) order.
// The text includes order items, total, and bank transfer instructions if a
// bankLine is provided; otherwise falls back to "bayar di kasir". Intended
// for customers who ordered online and need to transfer before pickup.
func FormatPendingInvoice(order *entity.Order, storeName, bankLine string) string {
	if storeName == "" {
		storeName = "Toko Bahan Kue Santi"
	}
	var b strings.Builder

	fmt.Fprintf(&b, "*%s*\n", strings.ToUpper(storeName))
	fmt.Fprintf(&b, "_Rincian Pesanan_\n\n")
	fmt.Fprintf(&b, "ID: #%s\n", shortOrderID(order.ID))
	fmt.Fprintf(&b, "Tanggal: %s\n", order.CreatedAt.In(jktLoc()).Format("02 Jan 2006, 15:04"))
	b.WriteString("─────────────────\n")

	if len(order.Items) > 0 {
		fmt.Fprintf(&b, "Item (%d):\n", len(order.Items))
		for _, item := range order.Items {
			line := item.UnitPrice * float64(item.Quantity)
			fmt.Fprintf(&b, "• %s\n", item.Name)
			fmt.Fprintf(&b, "   %d × %s = %s\n", item.Quantity, rp(item.UnitPrice), rp(line))
		}
	}
	b.WriteString("─────────────────\n")
	fmt.Fprintf(&b, "*TOTAL: %s*\n\n", rp(order.Total))

	if bankLine != "" {
		fmt.Fprintf(&b, "Silakan transfer ke:\n*%s*\n\n", bankLine)
		b.WriteString("Kirim bukti pembayaran ke nomor ini, pesanan akan kami proses setelah pembayaran diterima. Terima kasih 🙏")
	} else {
		b.WriteString("Silakan lakukan pembayaran di kasir atau hubungi admin untuk info rekening. Terima kasih 🙏")
	}

	return b.String()
}
