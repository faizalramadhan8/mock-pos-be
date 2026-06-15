package whatsapp

import (
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
)

// FormatReceipt renders a customer-facing receipt for WhatsApp. Mirrors the
// thermal struk Bu Santi cetak: header (nama/alamat/telp), info block (No #,
// Kasir, Tanggal, Member/Pelanggan), items, Subtotal / Hemat Member / Total,
// disclaimer. NO Diskon / PPN / Pembayaran / Kasir-bawah block — Bu Santi
// minta struk diperingkas.
func FormatReceipt(order *entity.Order, storeName, storeAddress, storePhone, cashierName string) string {
	if storeName == "" {
		storeName = "Toko Bahan Kue Santi"
	}
	if cashierName == "" {
		cashierName = "-"
	}
	var b strings.Builder

	// Header
	fmt.Fprintf(&b, "*%s*\n", storeName)
	if storeAddress != "" {
		fmt.Fprintf(&b, "%s\n", storeAddress)
	}
	if storePhone != "" {
		fmt.Fprintf(&b, "Telp. %s\n", storePhone)
	}
	b.WriteString("─────────────────\n")

	// Info block — orderNo format YYYY.MM.DD.NNNNN sama seperti struk offline.
	created := order.CreatedAt.In(jktLoc())
	digits := stripNonDigits(order.ID)
	if digits == "" {
		digits = order.ID
	}
	idTail := tailPad(digits, 5)
	orderNo := fmt.Sprintf("%04d.%02d.%02d.%s", created.Year(), int(created.Month()), created.Day(), idTail)
	dateStr := created.Format("02-01-2006  15:04")

	fmt.Fprintf(&b, "No #    : %s\n", orderNo)
	fmt.Fprintf(&b, "Kasir   : %s\n", cashierName)
	fmt.Fprintf(&b, "Tanggal : %s\n", dateStr)

	// Customer label: "Member" kalau pakai member, "Pelanggan" kalau non-member
	// yang isi nama. Skip kalau walk-in tanpa nama.
	customerName := ""
	customerLabel := "Pelanggan"
	if order.Member != nil && order.Member.Name != "" {
		customerName = order.Member.Name
		customerLabel = "Member"
	} else if order.Customer != "" {
		customerName = order.Customer
	}
	if customerName != "" {
		fmt.Fprintf(&b, "%s : %s\n", padRight(customerLabel, 7), customerName)
	}
	b.WriteString("─────────────────\n")

	// Items: 2-line per item (nama+qty di atas, harga indented di bawah) — WA
	// pakai variable-width font, single-line dengan column alignment tidak
	// reliable. Indent membuat harga gampang di-scan. Item yang ditebus pakai
	// poin di-tag "🎁" + value "−X poin" (bukan rupiah), dan tidak masuk
	// gross subtotal — mereka dilaporkan terpisah di bagian summary.
	var memberSavings, gross float64
	var pointsUsed int
	for _, item := range order.Items {
		lineTotal := item.UnitPrice * float64(item.Quantity)
		if item.RedeemedWithPoints {
			pointsUsed += int(lineTotal)
			fmt.Fprintf(&b, "🎁 %s ×%d\n", item.Name, item.Quantity)
			fmt.Fprintf(&b, "   −%s poin\n", thousand(int64(lineTotal)))
			continue
		}
		regular := item.UnitPrice
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			regular = *item.RegularPrice
			memberSavings += (regular - item.UnitPrice) * float64(item.Quantity)
		}
		gross += regular * float64(item.Quantity)
		fmt.Fprintf(&b, "%s ×%d\n", item.Name, item.Quantity)
		fmt.Fprintf(&b, "   %s\n", rp(lineTotal))
	}
	b.WriteString("─────────────────\n")

	// Summary
	fmt.Fprintf(&b, "Subtotal: %s\n", rp(gross))
	if memberSavings > 0 {
		fmt.Fprintf(&b, "Hemat Member: -%s\n", rp(memberSavings))
	}
	if pointsUsed > 0 {
		fmt.Fprintf(&b, "Poin Dipakai: -%s poin\n", thousand(int64(pointsUsed)))
	}
	fmt.Fprintf(&b, "*Total: %s*\n", rp(order.Total))

	// Poin diperoleh — query best-effort via member_point_movements diluar
	// fungsi ini (caller bisa kirim via parameter). Untuk WA, kita pakai
	// pre-computed kalau ada. Saat ini receipt.go tidak punya akses DB; jadi
	// kalau caller mau tampilkan, kirim via signature yang lebih kaya. Untuk
	// MVP, biarkan kosong di WA — tampil di FE struk modal saja.

	b.WriteString("─────────────────\n")
	b.WriteString("_Barang yang sudah dibeli tidak dapat ditukar atau dikembalikan._\n\n")
	b.WriteString("Terimakasih sudah berbelanja!")
	return b.String()
}

// thousand formats an integer with "." thousand separators (no Rp prefix).
func thousand(n int64) string {
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
		return "-" + string(out)
	}
	return string(out)
}

// stripNonDigits returns only the digit characters of s.
func stripNonDigits(s string) string {
	var b strings.Builder
	for _, c := range s {
		if c >= '0' && c <= '9' {
			b.WriteRune(c)
		}
	}
	return b.String()
}

// tailPad returns the last n chars of s, left-padded with '0' if shorter.
func tailPad(s string, n int) string {
	if len(s) >= n {
		return strings.ToUpper(s[len(s)-n:])
	}
	return strings.ToUpper(strings.Repeat("0", n-len(s)) + s)
}

// padRight pads s on the right with spaces to width w.
func padRight(s string, w int) string {
	if len(s) >= w {
		return s
	}
	return s + strings.Repeat(" ", w-len(s))
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
