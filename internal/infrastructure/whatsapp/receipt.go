package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
)

// FormatReceipt renders a compact, customer-friendly receipt for WhatsApp.
// Mirrors the printed thermal struk format Santi prefers: one line per item
// with name+qty on the left and line total on the right, then a summary block
// (Subtotal / Hemat / Total). The "harga member, hemat …" detail per-item
// was removed (customers found it cluttered) — savings are aggregated at the
// bottom instead.
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

	// Item lines — one row per item: "Name ×qty   Rp lineTotal".
	// The line total goes on its own line below the name, prefixed with a
	// soft indent, so long product names don't wrap into the price column.
	var memberSavings float64
	for _, item := range order.Items {
		lineTotal := item.UnitPrice * float64(item.Quantity)
		fmt.Fprintf(&b, "%s ×%d\n", item.Name, item.Quantity)
		fmt.Fprintf(&b, "   %s\n", rp(lineTotal))
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			memberSavings += (*item.RegularPrice - item.UnitPrice) * float64(item.Quantity)
		}
	}
	b.WriteString("─────────────────\n")

	// Summary: Subtotal → (Hemat Member) → (Diskon) → (PPN) → TOTAL
	gross := 0.0
	for _, it := range order.Items {
		gross += it.UnitPrice * float64(it.Quantity)
	}
	if memberSavings > 0 {
		fmt.Fprintf(&b, "Subtotal: %s\n", rp(gross+memberSavings))
		fmt.Fprintf(&b, "Hemat Member: -%s\n", rp(memberSavings))
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

	// Payment line(s) — single or split.
	if len(order.Payments) > 1 {
		b.WriteString("Pembayaran:\n")
		for _, p := range order.Payments {
			fmt.Fprintf(&b, "  %s: %s\n", strings.ToUpper(p.Method), rp(p.Amount))
		}
	} else {
		fmt.Fprintf(&b, "Pembayaran: %s\n", strings.ToUpper(order.Payment))
	}
	if cashierName != "" {
		fmt.Fprintf(&b, "Kasir: %s\n", cashierName)
	}
	b.WriteString("─────────────────\n")
	b.WriteString("_Barang yang dibeli tidak dapat ditukar atau dikembalikan._\n\n")
	b.WriteString("Terima kasih sudah berbelanja 🙏")
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
