package handler

import (
	"context"
	"fmt"
	"html"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/application/usecase"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type DeviceController struct {
	Log     *zerolog.Logger
	Service *usecase.DeviceService
	Configs *config.Config
}

// extractBaseURL derives the public-facing base URL from the incoming request.
// Works for the common setups in this app: direct access, nginx reverse proxy,
// and Cloudflare Tunnel (which sets the CF-Visitor header to signal https even
// when the tunnel leg to origin is plain http).
func extractBaseURL(c *fiber.Ctx) string {
	host := c.Hostname()
	if host == "" {
		return ""
	}
	// Cloudflare Tunnel / Cloudflare Proxy: CF-Visitor tells us the client-
	// side scheme. Most reliable for this deployment.
	if cv := c.Get("CF-Visitor"); strings.Contains(cv, `"scheme":"https"`) {
		return "https://" + host
	}
	// Standard reverse-proxy chain.
	if proto := c.Get("X-Forwarded-Proto"); proto != "" {
		return proto + "://" + host
	}
	// Direct TLS or plain HTTP (dev). Protocol() honors ProxyHeader config.
	return c.Protocol() + "://" + host
}

func NewDeviceController(ctx context.Context) *DeviceController {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	db := ctx.Value(enum.GormCtxKey).(*gorm.DB)
	return &DeviceController{
		Log:     logger,
		Service: usecase.NewDeviceService(ctx, db),
		Configs: configs,
	}
}

// GetStatus is polled by the frontend while waiting for owner approval.
// Query params: email (to resolve user_id without requiring auth) + fingerprint.
func (ctrl *DeviceController) GetStatus(c *fiber.Ctx) error {
	email := strings.TrimSpace(c.Query("email"))
	fingerprint := strings.TrimSpace(c.Query("fingerprint"))
	if email == "" || fingerprint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(dto.ApiResponse{
			Code:    fiber.StatusBadRequest,
			Message: "email and fingerprint are required",
		})
	}
	user, err := ctrl.Service.AuthR.FindByEmail(email)
	if err != nil {
		return c.JSON(dto.ApiResponse{
			Code:    fiber.StatusOK,
			Message: "successfully",
			Body:    dto.DeviceStatusResponse{Status: "unknown", Fingerprint: fingerprint},
		})
	}
	resp, fail := ctrl.Service.GetStatus(user.ID, fingerprint)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: resp})
}

// List devices for a given user (admin only).
func (ctrl *DeviceController) List(c *fiber.Ctx) error {
	userID := c.Params("id")
	devices, fail := ctrl.Service.ListByUser(userID)
	if fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "successfully", Body: devices})
}

// Revoke deletes a trusted device record (admin only).
func (ctrl *DeviceController) Revoke(c *fiber.Ctx) error {
	deviceID := c.Params("device_id")
	if fail := ctrl.Service.Revoke(deviceID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Device revoked"})
}

// EmergencyApprove is the manual fallback when WA links don't work
// (superadmin only).
func (ctrl *DeviceController) EmergencyApprove(c *fiber.Ctx) error {
	deviceID := c.Params("device_id")
	if fail := ctrl.Service.EmergencyApprove(deviceID); fail != nil {
		return c.Status(fail.StatusCode.Code).JSON(dto.ApiResponse{
			Code:    fail.StatusCode.Code,
			Message: fail.StatusCode.Message,
			Error:   fail.Message,
		})
	}
	return c.JSON(dto.ApiResponse{Code: fiber.StatusOK, Message: "Device approved"})
}

// ApproveLink handles GET /auth/devices/approve?t=<token> — clicked by owner
// from the WhatsApp approval message. Renders a styled HTML confirmation.
func (ctrl *DeviceController) ApproveLink(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Query("t"))
	if token == "" {
		return renderPage(c, fiber.StatusBadRequest, pageData{
			Kind:  "error",
			Title: "Link tidak valid",
			Body:  "Token pada URL kosong. Buka ulang link dari WhatsApp.",
		})
	}
	_, user, err := ctrl.Service.ApproveByCode(token)
	if err != nil {
		ctrl.Log.Warn().Err(err).Msg("approve via link failed")
		return renderPage(c, fiber.StatusBadRequest, pageData{
			Kind:  "error",
			Title: "Gagal approve",
			Body:  err.Error(),
		})
	}
	ctrl.Service.SendConfirmation(user, true)
	name := "Kasir"
	if user != nil {
		name = user.FullName
	}
	return renderPage(c, fiber.StatusOK, pageData{
		Kind:  "success",
		Title: "Device disetujui",
		Body: fmt.Sprintf(
			"Login untuk <b>%s</b> berhasil di-approve. Kasir sudah bisa melanjutkan login di device tersebut.",
			html.EscapeString(name),
		),
		Hint: "Halaman ini aman ditutup.",
	})
}

// RejectLink handles GET /auth/devices/reject?t=<token>.
func (ctrl *DeviceController) RejectLink(c *fiber.Ctx) error {
	token := strings.TrimSpace(c.Query("t"))
	if token == "" {
		return renderPage(c, fiber.StatusBadRequest, pageData{
			Kind:  "error",
			Title: "Link tidak valid",
			Body:  "Token pada URL kosong. Buka ulang link dari WhatsApp.",
		})
	}
	_, user, err := ctrl.Service.RejectByCode(token)
	if err != nil {
		ctrl.Log.Warn().Err(err).Msg("reject via link failed")
		return renderPage(c, fiber.StatusBadRequest, pageData{
			Kind:  "error",
			Title: "Gagal tolak",
			Body:  err.Error(),
		})
	}
	ctrl.Service.SendConfirmation(user, false)
	name := "Kasir"
	if user != nil {
		name = user.FullName
	}
	return renderPage(c, fiber.StatusOK, pageData{
		Kind:  "reject",
		Title: "Device ditolak",
		Body: fmt.Sprintf(
			"Login untuk <b>%s</b> sudah ditolak. Device ini tidak akan bisa login lagi sampai di-reset.",
			html.EscapeString(name),
		),
		Hint: "Halaman ini aman ditutup.",
	})
}

// ─── HTML rendering ───

type pageData struct {
	Kind  string // "success" | "reject" | "error"
	Title string
	Body  string // raw HTML (caller must already-escape names/etc.)
	Hint  string
}

// Inline SVG icons — monochrome, `currentColor` inherits from parent. All
// wrapped with role="img" + aria-label so screen readers announce meaningful
// names (not "check mark U+2713" or emoji variants).
const (
	svgBrand = `<svg class="brand-mark" viewBox="0 0 24 24" fill="none" role="img" aria-label="Logo Toko Bahan Kue Santi"><path d="M12 3C9.2 3 7 5.2 7 8c0 1.3.5 2.5 1.3 3.4L4 21h16l-4.3-9.6C16.5 10.5 17 9.3 17 8c0-2.8-2.2-5-5-5z" fill="currentColor" opacity=".9"/><path d="M9.5 8a2.5 2.5 0 015 0" stroke="#fff" stroke-width="1.5" stroke-linecap="round" fill="none"/></svg>`
	svgCheck = `<svg viewBox="0 0 24 24" fill="none" role="img" aria-label="Disetujui"><path d="M5 12.5l4.5 4.5L19 7.5" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"/></svg>`
	svgX     = `<svg viewBox="0 0 24 24" fill="none" role="img" aria-label="Ditolak"><path d="M7 7l10 10M17 7L7 17" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>`
	svgWarn  = `<svg viewBox="0 0 24 24" fill="none" role="img" aria-label="Peringatan"><path d="M12 4v9M12 17v2" stroke="currentColor" stroke-width="2.5" stroke-linecap="round"/></svg>`
)

// renderPage outputs a mobile-first confirmation page, branded as Toko Bahan
// Kue Santi. Safe to open inside WhatsApp's in-app browser on the owner's
// phone — no external assets, inline CSS + SVG, minimal JS.
func renderPage(c *fiber.Ctx, status int, d pageData) error {
	var accent, bar, icon, liveRole string
	switch d.Kind {
	case "success":
		accent = "#10B981"
		bar = "linear-gradient(90deg,#10B981,#34D399)"
		icon = svgCheck
		liveRole = "status"
	case "reject":
		accent = "#EF4444"
		bar = "linear-gradient(90deg,#EF4444,#F87171)"
		icon = svgX
		liveRole = "status"
	default:
		accent = "#A0673C"
		bar = "linear-gradient(90deg,#A0673C,#E8B088)"
		icon = svgWarn
		liveRole = "alert"
	}
	ts := time.Now().Format("02 Jan 2006 · 15:04")
	title := html.EscapeString(d.Title)
	hint := html.EscapeString(d.Hint)

	doc := fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<meta name="theme-color" content="#FBF7F2">
<meta name="robots" content="noindex">
<title>%s — Toko Bahan Kue Santi</title>
<style>
  *{box-sizing:border-box}
  html,body{margin:0;padding:0}
  body{font-family:-apple-system,BlinkMacSystemFont,"Segoe UI",system-ui,sans-serif;
       background:#FBF7F2;color:#2A1F16;min-height:100vh;
       display:flex;flex-direction:column;align-items:center;justify-content:center;
       padding:24px 20px;padding-bottom:max(24px,env(safe-area-inset-bottom));
       padding-top:max(24px,env(safe-area-inset-top));
       -webkit-font-smoothing:antialiased}
  .brand{display:flex;align-items:center;gap:10px;margin-bottom:20px;
         font-size:13px;font-weight:600;color:#6B5945;letter-spacing:.2px}
  .brand .dot{width:30px;height:30px;border-radius:9px;background:linear-gradient(135deg,#E8B088,#A0673C);
              display:grid;place-items:center;color:#fff;flex-shrink:0}
  .brand .brand-mark{width:18px;height:18px;color:#fff}
  .card{background:#fff;border-radius:24px;max-width:400px;width:100%%;
        box-shadow:0 6px 30px rgba(139,94,60,.08),0 1px 0 rgba(139,94,60,.04);
        overflow:hidden}
  .bar{height:4px;background:%s}
  .body{padding:32px 24px 28px;text-align:center}
  .icon{width:64px;height:64px;border-radius:50%%;display:grid;place-items:center;margin:0 auto 16px;
        background:%s22;color:%s}
  .icon svg{width:34px;height:34px}
  h1{margin:0 0 10px;font-size:22px;font-weight:700;letter-spacing:-.3px}
  .desc{margin:0;font-size:15px;color:#6B5945;line-height:1.55}
  .desc b{color:#2A1F16;font-weight:600}
  .meta{margin:20px 0 0;padding-top:16px;border-top:1px solid #F0E6D8;
        font-size:12px;color:#9E8670;display:flex;justify-content:space-between;gap:12px}
  .hint{margin-top:18px;font-size:12px;color:#9E8670}
  .btn{display:inline-block;margin-top:18px;padding:10px 24px;border-radius:999px;
       background:#2A1F16;color:#fff;font-size:13px;font-weight:600;
       text-decoration:none;border:0;cursor:pointer;min-height:40px}
  .btn:focus-visible{outline:2px solid %s;outline-offset:3px}
  @media (prefers-reduced-motion: no-preference){
    .card{animation:pop .3s ease-out}
    @keyframes pop{from{transform:scale(.96);opacity:0}to{transform:scale(1);opacity:1}}
  }
  @media (prefers-color-scheme: dark){
    body{background:#1E1610;color:#F4E8DA}
    .brand{color:#B8A088}
    .card{background:#2A1F16;box-shadow:0 6px 30px rgba(0,0,0,.4)}
    .desc{color:#B8A088}
    .desc b{color:#F4E8DA}
    .meta,.hint{color:#8A7563;border-color:#3A2E22}
    .btn{background:#F4E8DA;color:#2A1F16}
  }
</style>
</head>
<body>
  <div class="brand"><div class="dot">%s</div><span>Toko Bahan Kue Santi</span></div>
  <main class="card" role="%s" aria-live="polite">
    <div class="bar" aria-hidden="true"></div>
    <div class="body">
      <div class="icon" aria-hidden="true">%s</div>
      <h1>%s</h1>
      <p class="desc">%s</p>
      <div class="meta"><span>Security</span><time>%s</time></div>
      %s
      <button type="button" class="btn" onclick="window.close()">Tutup</button>
    </div>
  </main>
</body>
</html>`,
		title,
		bar,
		accent, accent,
		accent,
		svgBrand,
		liveRole,
		icon,
		title,
		d.Body,
		html.EscapeString(ts),
		hintBlock(hint),
	)
	c.Set(fiber.HeaderContentType, "text/html; charset=utf-8")
	c.Set("X-Content-Type-Options", "nosniff")
	return c.Status(status).SendString(doc)
}

func hintBlock(hint string) string {
	if hint == "" {
		return ""
	}
	return fmt.Sprintf(`<p class="hint">%s</p>`, hint)
}
