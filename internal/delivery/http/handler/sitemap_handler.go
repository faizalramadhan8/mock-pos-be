package handler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// SitemapController — serve /sitemap.xml dinamis dari DB.
// Include: home + kategori list + semua produk yang tayang di storefront.
// Google recrawl ~1-2 minggu; kalau produk baru, biasanya baru muncul di
// SERP ~1 minggu. Cache 1 jam via header supaya BE tidak thrash setiap
// bot request.
type SitemapController struct {
	Log *zerolog.Logger
	DB  *gorm.DB
}

func NewSitemapController(ctx context.Context) *SitemapController {
	return &SitemapController{
		Log: ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger),
		DB:  ctx.Value(enum.GormCtxKey).(*gorm.DB),
	}
}

func (ctrl *SitemapController) Sitemap(c *fiber.Ctx) error {
	base := "https://tbksanti.id"

	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">` + "\n")

	// Static high-priority URLs.
	addURL(&b, base+"/shop/", "daily", "1.0", time.Now())
	addURL(&b, base+"/shop/kategori", "weekly", "0.8", time.Now())

	// Kategori.
	var cats []entity.EcomCategory
	ctrl.DB.Where("deleted_at IS NULL AND is_active = 1").Find(&cats)
	for _, c := range cats {
		addURL(&b, base+"/shop/kategori/"+c.ID, "weekly", "0.7", c.UpdatedAt)
	}

	// Produk yang tayang di storefront (aktif + stock_ecom > 0 + ecom_is_available).
	var products []entity.Product
	ctrl.DB.Select("id, updated_at").
		Where("deleted_at IS NULL AND is_active = 1 AND ecom_is_available = 1 AND stock_ecom > 0").
		Find(&products)
	for _, p := range products {
		addURL(&b, base+"/shop/produk/"+p.ID, "weekly", "0.6", p.UpdatedAt)
	}

	b.WriteString(`</urlset>` + "\n")

	c.Set(fiber.HeaderContentType, "application/xml; charset=utf-8")
	// Cache 1 jam — sitemap tidak butuh real-time.
	c.Set(fiber.HeaderCacheControl, "public, max-age=3600")
	return c.SendString(b.String())
}

func addURL(b *strings.Builder, loc, changefreq, priority string, lastmod time.Time) {
	b.WriteString("  <url>\n")
	b.WriteString("    <loc>" + xmlEscape(loc) + "</loc>\n")
	if !lastmod.IsZero() {
		b.WriteString("    <lastmod>" + lastmod.UTC().Format("2006-01-02") + "</lastmod>\n")
	}
	b.WriteString(fmt.Sprintf("    <changefreq>%s</changefreq>\n", changefreq))
	b.WriteString(fmt.Sprintf("    <priority>%s</priority>\n", priority))
	b.WriteString("  </url>\n")
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
