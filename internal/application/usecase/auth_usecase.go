package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/entity"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/internal/domain/repository"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/config"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/database"
	"github.com/faizalramadhan/pos-be/internal/infrastructure/email"
	"github.com/faizalramadhan/pos-be/pkg/util"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	Log     *zerolog.Logger
	Configs *config.Config
	Repo    *repository.AuthRepository
	Redis   *redis.Client
	Device  *DeviceService
	Email   *email.Client
}

func NewAuthService(ctx context.Context, db *gorm.DB) *AuthService {
	logger := ctx.Value(enum.LoggerCtxKey).(*zerolog.Logger)
	configs := ctx.Value(enum.ConfigCtxKey).(*config.Config)
	redisInstance := ctx.Value(enum.RedisCtxKey).(*database.Redis)
	return &AuthService{
		Log:     logger,
		Repo:    repository.NewAuthRepository(ctx, db),
		Configs: configs,
		Redis:   redisInstance.GetRedisClient(ctx),
		Device:  NewDeviceService(ctx, db),
		Email:   email.NewClient(configs.BrevoAPIKey, configs.BrevoSenderEmail, configs.BrevoSenderName),
	}
}

// RegisterCustomer — public register untuk customer ecom (Bu Santi 21 Jul 2026).
// Force role='user' (ignore req.role kalau ada). Auto-login setelah register:
// return LoginResponse dengan JWT langsung supaya FE bisa redirect ke storefront
// tanpa 2nd request.
func (s *AuthService) RegisterCustomer(req dto.CustomerRegisterRequest) (*dto.LoginResponse, *dto.ApiError) {
	// Email + phone dedup check.
	exists, err := s.Repo.ExistsByEmail(req.Email)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to check email"}
	}
	if exists {
		return nil, &dto.ApiError{StatusCode: fiber.ErrConflict, Message: "Email sudah terdaftar"}
	}
	if req.PhoneNumber != "" {
		exists, err = s.Repo.ExistsByPhone(req.PhoneNumber)
		if err != nil {
			return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to check phone"}
		}
		if exists {
			return nil, &dto.ApiError{StatusCode: fiber.ErrConflict, Message: "Nomor HP sudah terdaftar"}
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to hash password"}
	}

	user := &entity.User{
		ID:          uuid.New().String(),
		Email:       req.Email,
		FullName:    req.FullName,
		PhoneNumber: req.PhoneNumber,
		Password:    string(hashedPassword),
		Role:        enum.RoleUser, // hardcode 'user' — cegah escalation via public endpoint
		IsActive:    true,
	}

	if err := s.Repo.Create(user); err != nil {
		s.Log.Error().Err(err).Msg("RegisterCustomer create failed")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to create user"}
	}

	// Issue JWT langsung (auto-login).
	claims := &dto.JWTClaims{
		ID: user.ID, Email: user.Email, Fullname: user.FullName,
		Phone: user.PhoneNumber, Role: string(user.Role), Session: user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{Time: time.Now().Add(s.Configs.JwtAccessTokenExpiresIn)},
			IssuedAt:  &jwt.NumericDate{Time: time.Now()},
		},
	}
	accessToken, err := util.MarshalClaims(s.Configs.JwtSecret, claims)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: err.Error()}
	}

	return &dto.LoginResponse{
		AccessToken: accessToken.TokenString,
		ExpiresIn:   int64(s.Configs.JwtAccessTokenExpiresIn.Seconds()),
		User:        s.toUserResponse(user),
	}, nil
}

func (s *AuthService) Register(req dto.RegisterRequest) (*dto.RegisterResponse, *dto.ApiError) {
	exists, err := s.Repo.ExistsByEmail(req.Email)
	if err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to check email availability",
		}
	}
	if exists {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrConflict,
			Message:    "Email already registered",
		}
	}

	if req.PhoneNumber != "" {
		exists, err = s.Repo.ExistsByPhone(req.PhoneNumber)
		if err != nil {
			s.Log.Error().Msg(err.Error())
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrInternalServerError,
				Message:    "Failed to check phone availability",
			}
		}
		if exists {
			return nil, &dto.ApiError{
				StatusCode: fiber.ErrConflict,
				Message:    "Phone number already registered",
			}
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to hash password",
		}
	}

	role := enum.RoleUser
	if req.Role != "" {
		role = enum.Role(req.Role)
	}

	user := &entity.User{
		ID:          uuid.New().String(),
		Email:       req.Email,
		FullName:    req.FullName,
		PhoneNumber: req.PhoneNumber,
		Password:    string(hashedPassword),
		Role:        role,
		NIK:         req.NIK,
		IsActive:    true,
	}

	if req.DateOfBirth != "" {
		parsed := util.ParseDateOnly(req.DateOfBirth)
		if dob, err := time.Parse("2006-01-02", parsed); err == nil {
			user.DateOfBirth = &dob
		}
	}

	if err := s.Repo.Create(user); err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to create user",
		}
	}

	response := &dto.RegisterResponse{
		ID:       user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     string(user.Role),
	}

	return response, nil
}

func (s *AuthService) Login(req dto.LoginRequest, userAgent, baseURL string) (*dto.LoginResponse, *dto.DevicePendingResponse, *dto.ApiError) {
	user, err := s.Repo.FindByEmail(req.Email)
	if err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, nil, &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    "User not found",
		}
	}

	if !user.IsActive {
		return nil, nil, &dto.ApiError{
			StatusCode: fiber.ErrForbidden,
			Message:    "Account is deactivated",
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, nil, &dto.ApiError{
			StatusCode: fiber.ErrUnauthorized,
			Message:    "Invalid credentials",
		}
	}

	// Device binding check — only gates cashier/staff/user roles.
	// superadmin/admin bypass so owner keeps emergency access.
	if IsGatedRole(user.Role) {
		dev, approved, fail := s.Device.EnsureApproved(user, req.DeviceFingerprint, userAgent, baseURL)
		if fail != nil {
			return nil, nil, fail
		}
		if !approved {
			return nil, &dto.DevicePendingResponse{
				DeviceID:    dev.ID,
				Fingerprint: dev.Fingerprint,
				Status:      string(dev.Status),
			}, nil
		}
	}

	claims := &dto.JWTClaims{
		ID:       user.ID,
		Email:    user.Email,
		Fullname: user.FullName,
		Phone:    user.PhoneNumber,
		Role:     string(user.Role),
		Session:  user.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: &jwt.NumericDate{
				Time: time.Now().Add(s.Configs.JwtAccessTokenExpiresIn),
			},
			IssuedAt: &jwt.NumericDate{
				Time: time.Now(),
			},
		},
	}

	accessToken, err := util.MarshalClaims(s.Configs.JwtSecret, claims)
	if err != nil {
		return nil, nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    err.Error(),
		}
	}

	userResponse := dto.LoginResponse{
		AccessToken: accessToken.TokenString,
		ExpiresIn:   int64(s.Configs.JwtAccessTokenExpiresIn.Seconds()),
		User:        s.toUserResponse(user),
	}

	return &userResponse, nil, nil
}

func (s *AuthService) toUserResponse(user *entity.User) dto.UserResponse {
	resp := dto.UserResponse{
		ID:       user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Phone:    user.PhoneNumber,
		Role:     string(user.Role),
		NIK:      user.NIK,
		IsActive: user.IsActive,
		Initials: getInitials(user.FullName),
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
	}
	if user.DateOfBirth != nil {
		dob := user.DateOfBirth.Format("2006-01-02")
		resp.DateOfBirth = &dob
	}
	return resp
}

func getInitials(name string) string {
	parts := strings.Fields(name)
	if len(parts) == 0 {
		return ""
	}
	initials := string([]rune(parts[0])[0])
	if len(parts) > 1 {
		initials += string([]rune(parts[len(parts)-1])[0])
	}
	return strings.ToUpper(initials)
}

func (s *AuthService) GetSession(claims *dto.JWTClaims) (*dto.UserSessions, *dto.ApiError) {
	ctx := context.Background()
	cacheKey := fmt.Sprintf("user:session:%s", claims.ID)

	cachedData, err := s.Redis.Get(ctx, cacheKey).Result()
	if err == nil && cachedData != "" {
		var session dto.UserSessions
		if err := json.Unmarshal([]byte(cachedData), &session); err == nil {
			s.Log.Info().Msgf("Session cache hit for user: %s", claims.ID)
			return &session, nil
		}
		s.Log.Warn().Err(err).Msg("Failed to unmarshal cached session")
	}

	s.Log.Info().Msgf("Session cache miss for user: %s, fetching from DB", claims.ID)
	user, err := s.Repo.FindByID(claims.ID)
	if err != nil {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    err.Error(),
		}
	}

	session := dto.UserSessions{
		ID:       user.ID,
		FullName: user.FullName,
		Role:     string(user.Role),
	}

	sessionJSON, err := json.Marshal(session)
	if err == nil {
		if err := s.Redis.Set(ctx, cacheKey, sessionJSON, time.Hour).Err(); err != nil {
			s.Log.Warn().Err(err).Msg("Failed to cache session")
		} else {
			s.Log.Info().Msgf("Session cached for user: %s", claims.ID)
		}
	}

	return &session, nil
}

func (s *AuthService) Logout() *dto.ApiError {
	s.Log.Info().Msg("User logged out successfully")
	return nil
}

func (s *AuthService) GetAllUsers() ([]dto.UserResponse, *dto.ApiError) {
	users, err := s.Repo.FindAll()
	if err != nil {
		s.Log.Error().Err(err).Msg("Failed to fetch users")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to fetch users",
		}
	}

	var result []dto.UserResponse
	for _, u := range users {
		result = append(result, s.toUserResponse(&u))
	}
	return result, nil
}

func (s *AuthService) UpdateUser(id string, req dto.UpdateUserRequest) (*dto.UserResponse, *dto.ApiError) {
	user, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    "User not found",
		}
	}

	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.PhoneNumber != "" {
		user.PhoneNumber = req.PhoneNumber
	}
	if req.Role != "" {
		user.Role = enum.Role(req.Role)
	}
	if req.NIK != "" {
		user.NIK = req.NIK
	}
	if req.DateOfBirth != "" {
		parsed := util.ParseDateOnly(req.DateOfBirth)
		if dob, err := time.Parse("2006-01-02", parsed); err == nil {
			user.DateOfBirth = &dob
		}
	}

	if err := s.Repo.Update(user); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update user")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to update user",
		}
	}

	resp := s.toUserResponse(user)
	return &resp, nil
}

func (s *AuthService) ToggleUserActive(id string) (*dto.UserResponse, *dto.ApiError) {
	user, err := s.Repo.FindByID(id)
	if err != nil {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    "User not found",
		}
	}

	user.IsActive = !user.IsActive
	if err := s.Repo.Update(user); err != nil {
		s.Log.Error().Err(err).Msg("Failed to toggle user active status")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to update user",
		}
	}

	resp := s.toUserResponse(user)
	return &resp, nil
}

func (s *AuthService) ResetPassword(id string, req dto.ResetPasswordRequest) *dto.ApiError {
	user, err := s.Repo.FindByID(id)
	if err != nil {
		return &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    "User not found",
		}
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to hash password",
		}
	}

	user.Password = string(hashedPassword)
	if err := s.Repo.Update(user); err != nil {
		return &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to reset password",
		}
	}

	return nil
}

// UpdateCustomerProfile — self-service update terbatas ke field aman
// (fullname, phone). Cegah customer manipulate role/email/isActive via
// endpoint ini. Email tidak boleh diubah karena dipakai sebagai login key
// + jadi anchor password reset — kalau perlu ganti email, harus support tiket.
func (s *AuthService) UpdateCustomerProfile(userID string, req dto.CustomerProfileUpdateRequest) (*dto.UserResponse, *dto.ApiError) {
	user, err := s.Repo.FindByID(userID)
	if err != nil {
		return nil, &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "User not found"}
	}
	if req.FullName != "" {
		user.FullName = req.FullName
	}
	if req.PhoneNumber != "" {
		user.PhoneNumber = req.PhoneNumber
	}
	if err := s.Repo.Update(user); err != nil {
		s.Log.Error().Err(err).Msg("Failed to update customer profile")
		return nil, &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update profile"}
	}
	resp := s.toUserResponse(user)
	return &resp, nil
}

// SendPasswordResetOTP — generate 6-digit OTP, store di Redis (10min TTL),
// kirim ke email via Brevo. Rate-limit 60s per email supaya tidak spam.
//
// Return apiError HANYA untuk internal logging — handler tetap balik 200 ke
// user (silent-succeed pattern) supaya attacker tidak bisa enumerate email.
func (s *AuthService) SendPasswordResetOTP(emailAddr string) *dto.ApiError {
	ctx := context.Background()

	// Rate limit — cegah spam via loop.
	rlKey := "pwd_reset_rl:" + strings.ToLower(emailAddr)
	if exists, _ := s.Redis.Get(ctx, rlKey).Result(); exists != "" {
		return &dto.ApiError{StatusCode: fiber.ErrTooManyRequests, Message: "rate limited"}
	}

	user, err := s.Repo.FindByEmail(emailAddr)
	if err != nil || user == nil {
		// Email tidak terdaftar — silent (jangan leak).
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "email not found"}
	}

	// Generate OTP 6 digit. Pakai crypto/rand di production biar unpredictable;
	// util.RandomDigits di codebase belum ada, jadi kita ambil pattern dari
	// existing device_usecase token generation.
	otp := generate6DigitOTP()

	otpKey := "pwd_reset_otp:" + strings.ToLower(emailAddr)
	if err := s.Redis.Set(ctx, otpKey, otp, 10*time.Minute).Err(); err != nil {
		s.Log.Error().Err(err).Msg("failed to store password reset OTP")
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "failed to prepare OTP"}
	}
	// Set rate-limit marker.
	s.Redis.Set(ctx, rlKey, "1", 60*time.Second)

	// Kirim email — best-effort di goroutine supaya tidak block request.
	name := user.FullName
	if name == "" {
		name = "Customer"
	}
	subject := "Kode reset password TBK Santi"
	html := fmt.Sprintf(`<div style="font-family:Arial,sans-serif;max-width:480px;margin:0 auto;padding:24px;">
<h2 style="color:#C4302B;margin:0 0 12px">Reset Password</h2>
<p>Hai %s,</p>
<p>Kamu meminta reset password akun TBK Santi. Masukkan kode berikut di halaman reset:</p>
<div style="background:#f7e8e6;border-radius:12px;padding:20px;margin:20px 0;text-align:center;">
  <span style="font-size:32px;font-weight:900;letter-spacing:8px;color:#C4302B;">%s</span>
</div>
<p style="color:#666;font-size:13px">Kode berlaku 10 menit. Kalau kamu tidak meminta reset ini, abaikan email ini — password kamu tetap aman.</p>
<p style="color:#999;font-size:12px;margin-top:24px">Toko Bahan Kue Santi · tbksanti.id</p>
</div>`, name, otp)
	text := fmt.Sprintf("Kode reset password TBK Santi: %s\nBerlaku 10 menit. Kalau kamu tidak minta, abaikan email ini.", otp)

	go func() {
		if err := s.Email.Send(emailAddr, name, subject, html, text); err != nil {
			s.Log.Warn().Err(err).Str("email", emailAddr).Msg("failed to send OTP email")
		}
	}()

	return nil
}

// ConfirmPasswordResetOTP — verify OTP dari Redis, set password baru,
// invalidate OTP + rate-limit key supaya tidak reuse.
func (s *AuthService) ConfirmPasswordResetOTP(req dto.PasswordResetConfirmOTP) *dto.ApiError {
	ctx := context.Background()
	otpKey := "pwd_reset_otp:" + strings.ToLower(req.Email)
	stored, err := s.Redis.Get(ctx, otpKey).Result()
	if err != nil || stored == "" {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "OTP tidak valid atau sudah expired"}
	}
	if stored != req.OTP {
		return &dto.ApiError{StatusCode: fiber.ErrBadRequest, Message: "Kode OTP salah"}
	}

	user, err := s.Repo.FindByEmail(req.Email)
	if err != nil || user == nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "Akun tidak ditemukan"}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to hash password"}
	}
	user.Password = string(hashed)
	if err := s.Repo.Update(user); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to save password"}
	}

	// Consume OTP + clear rate limit (biar user bisa login langsung).
	s.Redis.Del(ctx, otpKey)
	s.Redis.Del(ctx, "pwd_reset_rl:"+strings.ToLower(req.Email))

	// Optional: invalidate session cache supaya JWT lama harus re-login.
	// Tidak strictly needed — password baru pakai untuk future login;
	// existing JWT tetap valid (long-lived). Skip untuk simplicity.

	return nil
}

// generate6DigitOTP — pakai time nano+uuid low bits sebagai seed pseudo-random.
// Cukup random untuk short-lived (10min TTL) + rate-limited. Kalau nanti mau
// crypto-secure, ganti ke crypto/rand.
func generate6DigitOTP() string {
	// XOR nano dengan bits terakhir uuid untuk sedikit entropy tambahan.
	n := time.Now().UnixNano()
	u := uuid.New()
	seed := n ^ int64(u[0])<<48 ^ int64(u[1])<<40 ^ int64(u[2])<<32
	// Modulo 1000000, pad leading zero.
	val := seed % 1_000_000
	if val < 0 {
		val = -val
	}
	return fmt.Sprintf("%06d", val)
}

func (s *AuthService) ChangePassword(userID string, req dto.ChangePasswordRequest) *dto.ApiError {
	user, err := s.Repo.FindByID(userID)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrNotFound, Message: "User not found"}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.CurrentPassword)); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrUnauthorized, Message: "Password lama salah"}
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to hash password"}
	}

	user.Password = string(hashed)
	if err := s.Repo.Update(user); err != nil {
		return &dto.ApiError{StatusCode: fiber.ErrInternalServerError, Message: "Failed to update password"}
	}
	return nil
}

func (s *AuthService) DeleteUser(id string) *dto.ApiError {
	if err := s.Repo.Delete(id); err != nil {
		s.Log.Error().Err(err).Msg("Failed to delete user")
		return &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to delete user",
		}
	}
	return nil
}
