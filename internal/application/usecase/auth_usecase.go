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
	}
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

func (s *AuthService) Login(req dto.LoginRequest) (*dto.LoginResponse, *dto.ApiError) {
	user, err := s.Repo.FindByEmail(req.Email)
	if err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrNotFound,
			Message:    "User not found",
		}
	}

	if !user.IsActive {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrForbidden,
			Message:    "Account is deactivated",
		}
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		s.Log.Error().Msg(err.Error())
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrUnauthorized,
			Message:    "Invalid credentials",
		}
	}

	ctx := context.Background()

	// Generate access token (short-lived: 15 minutes)
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
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    err.Error(),
		}
	}

	// Generate refresh token (long-lived: 7 days)
	refreshToken := uuid.New().String()
	refreshTokenData := &dto.RefreshTokenData{
		UserID:   user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     string(user.Role),
	}

	// Store refresh token in Redis
	if err := s.storeRefreshToken(ctx, refreshToken, refreshTokenData, s.Configs.JwtRefreshTokenExpiresIn); err != nil {
		s.Log.Error().Err(err).Msg("Failed to store refresh token")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to create session",
		}
	}

	userResponse := dto.LoginResponse{
		AccessToken:  accessToken.TokenString,
		RefreshToken: refreshToken,
		ExpiresIn:    int64(s.Configs.JwtAccessTokenExpiresIn.Seconds()),
		User:         s.toUserResponse(user),
	}

	return &userResponse, nil
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

func (s *AuthService) storeRefreshToken(ctx context.Context, token string, data *dto.RefreshTokenData, ttl time.Duration) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return s.Redis.Set(ctx, key, jsonData, ttl).Err()
}

func (s *AuthService) getRefreshTokenData(ctx context.Context, token string) (*dto.RefreshTokenData, error) {
	key := fmt.Sprintf("refresh_token:%s", token)
	data, err := s.Redis.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	var tokenData dto.RefreshTokenData
	if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
		return nil, err
	}
	return &tokenData, nil
}

func (s *AuthService) revokeRefreshToken(ctx context.Context, token string) error {
	key := fmt.Sprintf("refresh_token:%s", token)
	return s.Redis.Del(ctx, key).Err()
}

func (s *AuthService) revokeAllUserRefreshTokens(ctx context.Context, userID string) error {
	pattern := fmt.Sprintf("refresh_token:*")
	iter := s.Redis.Scan(ctx, 0, pattern, 0).Iterator()

	for iter.Next(ctx) {
		key := iter.Val()
		data, err := s.Redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}

		var tokenData dto.RefreshTokenData
		if err := json.Unmarshal([]byte(data), &tokenData); err != nil {
			continue
		}

		if tokenData.UserID == userID {
			s.Redis.Del(ctx, key)
		}
	}

	return iter.Err()
}

func (s *AuthService) RefreshToken(req dto.RefreshTokenRequest) (*dto.RefreshTokenResponse, *dto.ApiError) {
	ctx := context.Background()

	tokenData, err := s.getRefreshTokenData(ctx, req.RefreshToken)
	if err != nil {
		s.Log.Warn().Err(err).Msg("Invalid or expired refresh token")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrUnauthorized,
			Message:    "Invalid or expired refresh token",
		}
	}

	user, err := s.Repo.FindByID(tokenData.UserID)
	if err != nil {
		s.Log.Error().Err(err).Msg("User not found during token refresh")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrUnauthorized,
			Message:    "User not found",
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

	newAccessToken, err := util.MarshalClaims(s.Configs.JwtSecret, claims)
	if err != nil {
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    err.Error(),
		}
	}

	newRefreshToken := uuid.New().String()
	refreshTokenData := &dto.RefreshTokenData{
		UserID:   user.ID,
		Email:    user.Email,
		FullName: user.FullName,
		Role:     string(user.Role),
	}

	if err := s.storeRefreshToken(ctx, newRefreshToken, refreshTokenData, s.Configs.JwtRefreshTokenExpiresIn); err != nil {
		s.Log.Error().Err(err).Msg("Failed to store new refresh token")
		return nil, &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to refresh session",
		}
	}

	if err := s.revokeRefreshToken(ctx, req.RefreshToken); err != nil {
		s.Log.Warn().Err(err).Msg("Failed to revoke old refresh token")
	}

	s.Log.Info().Msgf("Token refreshed for user: %s", user.ID)

	return &dto.RefreshTokenResponse{
		AccessToken:  newAccessToken.TokenString,
		RefreshToken: newRefreshToken,
		ExpiresIn:    int64(s.Configs.JwtAccessTokenExpiresIn.Seconds()),
	}, nil
}

func (s *AuthService) Logout(refreshToken string) *dto.ApiError {
	ctx := context.Background()

	if err := s.revokeRefreshToken(ctx, refreshToken); err != nil {
		s.Log.Error().Err(err).Msg("Failed to revoke refresh token")
		return &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to logout",
		}
	}

	s.Log.Info().Msg("User logged out successfully")
	return nil
}

func (s *AuthService) LogoutAll(userID string) *dto.ApiError {
	ctx := context.Background()

	if err := s.revokeAllUserRefreshTokens(ctx, userID); err != nil {
		s.Log.Error().Err(err).Msg("Failed to revoke all user tokens")
		return &dto.ApiError{
			StatusCode: fiber.ErrInternalServerError,
			Message:    "Failed to logout from all devices",
		}
	}

	s.Log.Info().Msgf("User %s logged out from all devices", userID)
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
