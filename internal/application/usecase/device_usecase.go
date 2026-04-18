package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/whatsapp"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

// Roles that are subject to device binding. Admins/superadmins bypass — they
// need unrestricted access in case of emergency (e.g. WAHA down).
var deviceGatedRoles = map[enum.Role]bool{
	enum.RoleCashier: true,
	enum.RoleStaff:   true,
	enum.RoleUser:    true,
}

// Minimum gap between resending approval WA for the same pending device.
const notifyCooldown = 60 * time.Second

// Approval code TTL.
const codeTTL = 10 * time.Minute

type DeviceService struct {
	Log      *zerolog.Logger
	Configs  *config.Config
	DeviceR  *repository.DeviceRepository
	AuthR    *repository.AuthRepository
	WA       *whatsapp.Service
}

func NewDeviceService(ctx context.Context, db *gorm.DB) *DeviceService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	wa, _ := ctx.Value(enum.WhatsAppCtxKey).(*whatsapp.Service)
	return &DeviceService{
		Log:     logger,
		Configs: configs,
		DeviceR: repository.NewDeviceRepository(ctx, db),
		AuthR:   repository.NewAuthRepository(ctx, db),
		WA:      wa,
	}
}

// IsGatedRole reports whether the given role must go through device approval.
func IsGatedRole(role enum.Role) bool { return deviceGatedRoles[role] }

// EnsureApproved checks the device for the given user. Returns (approved=true)
// if login may proceed, or (approved=false, pending) if the caller should
// return HTTP 202 and wait for owner approval.
// Side effects: creates a pending record + sends WA on first sight, re-sends
// WA if an existing pending record hasn't been re-notified recently.
func (s *DeviceService) EnsureApproved(user *entity.User, fingerprint, userAgent, baseURL string) (*entity.TrustedDevice, bool, *dto.ApiError) {
	if fingerprint == "" {
		return nil, false, &dto.ApiError{
			StatusCode: fiber.ErrBadRequest,
			Message:    "device_fingerprint is required for this role",
		}
	}

	dev, err := s.DeviceR.FindByUserAndFingerprint(user.ID, fingerprint)
	if err == nil {
		switch dev.Status {
		case entity.DeviceStatusApproved:
			_ = s.DeviceR.MarkUsed(dev.ID)
			return dev, true, nil
		case entity.DeviceStatusRejected:
			return dev, false, &dto.ApiError{
				StatusCode: fiber.ErrForbidden,
				Message:    "Device ditolak. Hubungi owner.",
			}
		case entity.DeviceStatusPending:
			s.maybeResendNotification(dev, user, baseURL)
			return dev, false, nil
		}
	}

	// Not found → create pending record and notify.
	code := newApprovalToken()
	expires := time.Now().Add(codeTTL)
	dev = &entity.TrustedDevice{
		ID:            uuid.New().String(),
		UserID:        user.ID,
		Fingerprint:   fingerprint,
		Status:        entity.DeviceStatusPending,
		ApprovalCode:  code,
		CodeExpiresAt: &expires,
		UserAgent:     truncate(userAgent, 255),
	}
	if err := s.DeviceR.Create(dev); err != nil {
		s.Log.Error().Err(err).Msg("Failed to create pending device record")
		return nil, false, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to register device",
		}
	}
	s.notifyOwner(dev, user, baseURL)
	return dev, false, nil
}

// ApproveByCode marks a pending device as approved. Returns the updated device
// + owning user, or an error suitable for replying to the webhook sender.
func (s *DeviceService) ApproveByCode(code string) (*entity.TrustedDevice, *entity.User, error) {
	dev, err := s.DeviceR.FindByApprovalCode(strings.TrimSpace(code))
	if err != nil {
		return nil, nil, fmt.Errorf("kode tidak ditemukan")
	}
	if dev.Status != entity.DeviceStatusPending {
		return dev, nil, fmt.Errorf("device sudah di-proses sebelumnya (status: %s)", dev.Status)
	}
	if dev.CodeExpiresAt != nil && time.Now().After(*dev.CodeExpiresAt) {
		return dev, nil, fmt.Errorf("kode sudah kadaluarsa, kasir harus login ulang")
	}

	now := time.Now()
	dev.Status = entity.DeviceStatusApproved
	dev.ApprovedAt = &now
	dev.ApprovalCode = ""
	dev.CodeExpiresAt = nil
	if err := s.DeviceR.Update(dev); err != nil {
		return dev, nil, fmt.Errorf("gagal menyimpan: %v", err)
	}

	user, err := s.AuthR.FindByID(dev.UserID)
	if err != nil {
		return dev, nil, fmt.Errorf("user tidak ditemukan")
	}
	return dev, user, nil
}

// RejectByCode marks a pending device as rejected.
func (s *DeviceService) RejectByCode(code string) (*entity.TrustedDevice, *entity.User, error) {
	dev, err := s.DeviceR.FindByApprovalCode(strings.TrimSpace(code))
	if err != nil {
		return nil, nil, fmt.Errorf("kode tidak ditemukan")
	}
	if dev.Status != entity.DeviceStatusPending {
		return dev, nil, fmt.Errorf("device sudah di-proses sebelumnya (status: %s)", dev.Status)
	}
	dev.Status = entity.DeviceStatusRejected
	dev.ApprovalCode = ""
	dev.CodeExpiresAt = nil
	if err := s.DeviceR.Update(dev); err != nil {
		return dev, nil, fmt.Errorf("gagal menyimpan: %v", err)
	}
	user, _ := s.AuthR.FindByID(dev.UserID)
	return dev, user, nil
}

// GetStatus is used by the frontend polling loop while waiting for approval.
func (s *DeviceService) GetStatus(userID, fingerprint string) (*dto.DeviceStatusResponse, *dto.ApiError) {
	dev, err := s.DeviceR.FindByUserAndFingerprint(userID, fingerprint)
	if err != nil {
		return &dto.DeviceStatusResponse{Status: "unknown", Fingerprint: fingerprint}, nil
	}
	return &dto.DeviceStatusResponse{Status: string(dev.Status), Fingerprint: fingerprint}, nil
}

// ListByUser returns all devices owned by a user.
func (s *DeviceService) ListByUser(userID string) ([]dto.DeviceResponse, *dto.ApiError) {
	devs, err := s.DeviceR.FindByUser(userID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	out := make([]dto.DeviceResponse, 0, len(devs))
	for _, d := range devs {
		out = append(out, toDeviceResponse(d))
	}
	return out, nil
}

func (s *DeviceService) Revoke(id string) *dto.ApiError {
	if err := s.DeviceR.Delete(id); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	return nil
}

// EmergencyApprove is used by superadmin to approve a device directly, e.g.
// when WAHA is unavailable.
func (s *DeviceService) EmergencyApprove(id string) *dto.ApiError {
	dev := &entity.TrustedDevice{ID: id}
	if err := s.DeviceR.DB.First(dev, "id = ?", id).Error; err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Device not found"}
	}
	now := time.Now()
	dev.Status = entity.DeviceStatusApproved
	dev.ApprovedAt = &now
	dev.ApprovalCode = ""
	dev.CodeExpiresAt = nil
	if err := s.DeviceR.Update(dev); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}
	return nil
}

// ─── WA notification helpers ───

// broadcastToAdmins sends the same text to every active admin/superadmin with
// a phone number. Returns the list of recipients it successfully reached.
func (s *DeviceService) broadcastToAdmins(text string) []entity.User {
	if s.WA == nil || !s.WA.Enabled() {
		return nil
	}
	admins, err := s.AuthR.FindAdmins()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to load admin list for WA broadcast")
		return nil
	}
	if len(admins) == 0 {
		s.Log.Warn().Msg("No admin/superadmin with phone found; WA notification skipped")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	sent := make([]entity.User, 0, len(admins))
	for _, a := range admins {
		if err := s.WA.SendText(ctx, a.PhoneNumber, text); err != nil {
			s.Log.Warn().Err(err).
				Str("admin_id", a.ID).
				Str("admin_phone", a.PhoneNumber).
				Msg("Failed to send WA to admin")
			continue
		}
		sent = append(sent, a)
	}
	return sent
}

func (s *DeviceService) notifyOwner(dev *entity.TrustedDevice, user *entity.User, baseURL string) {
	// Prefer request-derived URL (works behind Cloudflare + nginx);
	// fall back to APP_URL env override for setups where auto-detect fails.
	base := strings.TrimRight(baseURL, "/")
	if base == "" {
		base = strings.TrimRight(s.Configs.AppURL, "/")
	}
	if base == "" {
		s.Log.Warn().Msg("No base URL (request nor APP_URL); approval links will not be clickable")
	}
	approveURL := fmt.Sprintf("%s/api/v1/auth/devices/approve?t=%s", base, dev.ApprovalCode)
	rejectURL := fmt.Sprintf("%s/api/v1/auth/devices/reject?t=%s", base, dev.ApprovalCode)

	msg := fmt.Sprintf(
		"🔒 *Toko Bahan Kue Santi — Security*\n\n%s mencoba login dari device baru.\nWaktu: %s\n\n✅ Approve: %s\n❌ Tolak: %s\n\nLink berlaku 10 menit.",
		user.FullName,
		time.Now().Format("02 Jan 2006, 15:04"),
		approveURL,
		rejectURL,
	)

	sent := s.broadcastToAdmins(msg)
	if len(sent) == 0 {
		return
	}
	now := time.Now()
	dev.LastNotifiedAt = &now
	_ = s.DeviceR.Update(dev)
}

func (s *DeviceService) maybeResendNotification(dev *entity.TrustedDevice, user *entity.User, baseURL string) {
	if dev.LastNotifiedAt != nil && time.Since(*dev.LastNotifiedAt) < notifyCooldown {
		return
	}
	// Code may have expired; rotate it so kasir doesn't have to wait.
	if dev.CodeExpiresAt == nil || time.Now().After(*dev.CodeExpiresAt) {
		dev.ApprovalCode = newApprovalToken()
		expires := time.Now().Add(codeTTL)
		dev.CodeExpiresAt = &expires
		_ = s.DeviceR.Update(dev)
	}
	s.notifyOwner(dev, user, baseURL)
}

// SendConfirmation replies to all admins after one of them approves/rejects.
func (s *DeviceService) SendConfirmation(user *entity.User, approved bool) {
	name := "device"
	if user != nil {
		name = user.FullName
	}
	var msg string
	if approved {
		msg = fmt.Sprintf("✅ Device untuk %s berhasil di-approve. Kasir sudah bisa login.", name)
	} else {
		msg = fmt.Sprintf("❌ Device untuk %s sudah ditolak.", name)
	}
	s.broadcastToAdmins(msg)
}

// ─── helpers ───

// newApprovalToken returns a 32-char hex token (16 bytes of entropy).
// Long enough to be safe in a public URL, short enough to keep the link
// copy-pasteable in WhatsApp.
func newApprovalToken() string {
	buf := make([]byte, 16)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func toDeviceResponse(d entity.TrustedDevice) dto.DeviceResponse {
	r := dto.DeviceResponse{
		ID:        d.ID,
		UserID:    d.UserID,
		Status:    string(d.Status),
		Name:      d.Name,
		UserAgent: d.UserAgent,
		CreatedAt: d.CreatedAt.Format(time.RFC3339),
	}
	if d.ApprovedAt != nil {
		s := d.ApprovedAt.Format(time.RFC3339)
		r.ApprovedAt = &s
	}
	if d.LastUsedAt != nil {
		s := d.LastUsedAt.Format(time.RFC3339)
		r.LastUsedAt = &s
	}
	return r
}
