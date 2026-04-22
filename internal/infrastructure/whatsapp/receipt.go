package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
)

// FormatReceipt renders a compact human-readable receipt suitable for sending
// over WhatsApp as plain text. Formatting uses WhatsApp conventions (*bold*,
// _italic_) and monospaced blocks where appropriate.
//
// storeName is taken from settings so the message looks branded.
func FormatReceipt(order *entity.Order, storeName, cashierName string) string {
	if storeName == "" {
		storeName = "Toko Bahan Kue Santi"
	}
	var b strings.Builder

	fmt.Fprintf(&b, "*%s*\n", strings.ToUpper(storeName))
	fmt.Fprintf(&b, "_Struk Pembelian_\n\n")
	fmt.Fprintf(&b, "Tanggal: %s\n", order.CreatedAt.In(jktLoc()).Format("02 Jan 2006, 15:04"))
	fmt.Fprintf(&b, "ID: #%s\n", shortOrderID(order.ID))
	b.WriteString("─────────────────\n")

	var memberSavings float64
	fmt.Fprintf(&b, "Item (%d):\n", len(order.Items))
	for _, item := range order.Items {
		lineTotal := item.UnitPrice * float64(item.Quantity)
		fmt.Fprintf(&b, "• %s\n", item.Name)
		fmt.Fprintf(&b, "   %d × %s = %s\n",
			item.Quantity,
			rp(item.UnitPrice),
			rp(lineTotal),
		)
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			saved := (*item.RegularPrice - item.UnitPrice) * float64(item.Quantity)
			memberSavings += saved
			fmt.Fprintf(&b, "   _(harga member, hemat %s dari %s)_\n",
				rp(saved),
				rp(*item.RegularPrice),
			)
		}
		if item.DiscountAmount > 0 {
			fmt.Fprintf(&b, "   _diskon: -%s_\n", rp(item.DiscountAmount))
		}
	}
	b.WriteString("─────────────────\n")

	gross := 0.0
	for _, it := range order.Items {
		gross += it.UnitPrice * float64(it.Quantity)
	}
	if memberSavings > 0 {
		fmt.Fprintf(&b, "Subtotal: %s\n", rp(gross+memberSavings))
		fmt.Fprintf(&b, "💎 Hemat member: -%s\n", rp(memberSavings))
	} else {
		fmt.Fprintf(&b, "Subtotal: %s\n", rp(gross))
	}
	if order.OrderDiscount > 0 {
		fmt.Fprintf(&b, "Diskon: -%s\n", rp(order.OrderDiscount))
	}
	if order.PPN > 0 {
		fmt.Fprintf(&b, "PPN %.0f%%: %s\n", order.PPNRate, rp(order.PPN))
	}
	fmt.Fprintf(&b, "*TOTAL: %s*\n", rp(order.Total))
	fmt.Fprintf(&b, "Pembayaran: %s\n", strings.ToUpper(order.Payment))
	if cashierName != "" {
		fmt.Fprintf(&b, "Kasir: %s\n", cashierName)
	}
	b.WriteString("─────────────────\n")
	b.WriteString("Terima kasih telah berbelanja! 🙏")
	return b.String()
}

func rp(v float64) string {
	// 1.234.567 format
	n := int64(v + 0.5)
	neg := n < 0
	if neg {
		n = -n
	}
	s := fmt.Sprintf("%d", n)
	out := make([]byte, 0, len(s)+len(s)/3)
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, '.')
		}
		out = append(out, byte(c))
	}
	if neg {
		return "Rp -" + string(out)
	}
	return "Rp " + string(out)
}

// shortOrderID returns the last 8 characters of an order ID, uppercased.
// Used for customer-facing receipts where the full UUID is too long.
func shortOrderID(id string) string {
	if len(id) <= 8 {
		return strings.ToUpper(id)
	}
	return strings.ToUpper(id[len(id)-8:])
}

func jktLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.UTC
	}
	return loc
}
