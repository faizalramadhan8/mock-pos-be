package whatsapp

import (
	"fmt"
	"strings"

	"github.com/faizalramadhan/pos-be/internal/domain/entity"
)

// FormatPendingInvoice renders a WA message for a pending (unpaid) order.
// Format mirror struk offline / FormatReceipt: header (nama/alamat/telp),
// info block (No #, Tanggal, Pelanggan), items, Subtotal/Hemat/Total. Bedanya
// dengan FormatReceipt biasa: tambahan instruksi transfer bank (atau bayar
// di kasir kalau bankLine kosong) sebelum disclaimer.
func FormatPendingInvoice(order *entity.Order, storeName, storeAddress, storePhone, bankLine string) string {
	if storeName == "" {
		storeName = "Toko Bahan Kue Santi"
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
	fmt.Fprintf(&b, "Tanggal : %s\n", dateStr)
	fmt.Fprintf(&b, "Status  : *Belum Lunas*\n")

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

	// Items: 2-line per item, sama dengan FormatReceipt.
	var memberSavings, gross float64
	for _, item := range order.Items {
		regular := item.UnitPrice
		if item.RegularPrice != nil && *item.RegularPrice > item.UnitPrice {
			regular = *item.RegularPrice
			memberSavings += (regular - item.UnitPrice) * float64(item.Quantity)
		}
		gross += regular * float64(item.Quantity)

		lineTotal := item.UnitPrice * float64(item.Quantity)
		fmt.Fprintf(&b, "%s ×%d\n", item.Name, item.Quantity)
		fmt.Fprintf(&b, "   %s\n", rp(lineTotal))
	}
	b.WriteString("─────────────────\n")

	fmt.Fprintf(&b, "Subtotal: %s\n", rp(gross))
	if memberSavings > 0 {
		fmt.Fprintf(&b, "Hemat Member: -%s\n", rp(memberSavings))
	}
	fmt.Fprintf(&b, "*Total: %s*\n", rp(order.Total))
	b.WriteString("─────────────────\n")

	// Pending-specific: instruksi pembayaran.
	if bankLine != "" {
		fmt.Fprintf(&b, "Silakan transfer ke:\n*%s*\n\n", bankLine)
		b.WriteString("Kirim bukti pembayaran ke nomor ini. Pesanan akan diproses setelah pembayaran diterima.\n\n")
	} else {
		b.WriteString("Silakan lakukan pembayaran di kasir atau hubungi admin untuk info rekening.\n\n")
	}
	b.WriteString("_Barang yang sudah dibeli tidak dapat ditukar atau dikembalikan._\n\n")
	b.WriteString("Terimakasih sudah berbelanja!")
	return b.String()
}
