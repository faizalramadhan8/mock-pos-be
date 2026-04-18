package dto

type RegisterRequest struct {
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=8"`
	FullName    string `json:"fullname" validate:"required,min=3"`
	PhoneNumber string `json:"phone" validate:"omitempty"`
	Role        string `json:"role" validate:"omitempty,oneof=user admin superadmin cashier staff"`
	NIK         string `json:"nik" validate:"omitempty"`
	DateOfBirth string `json:"date_of_birth" validate:"omitempty"`
}

type RegisterResponse struct {
	ID       string `json:"id"`
	Email    string `json:"email"`
	FullName string `json:"fullname"`
	Role     string `json:"role"`
}

type LoginRequest struct {
	Email    string `json:"email,omitempty" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	ExpiresIn    int64        `json:"expires_in"`
	User         UserResponse `json:"user"`
}

type UserResponse struct {
	ID          string  `json:"id"`
	Email       string  `json:"email"`
	FullName    string  `json:"fullname"`
	Phone       string  `json:"phone,omitempty"`
	Role        string  `json:"role"`
	NIK         string  `json:"nik,omitempty"`
	DateOfBirth *string `json:"date_of_birth,omitempty"`
	IsActive    bool    `json:"is_active"`
	Initials    string  `json:"initials"`
	CreatedAt   string  `json:"created_at"`
}

type UserSessions struct {
	ID       string `json:"id"`
	FullName string `json:"fullname"`
	Role     string `json:"role,omitempty"`
	Email    string `json:"email,omitempty"`
	IsActive bool   `json:"is_active"`
}

type UpdateUserRequest struct {
	FullName    string `json:"fullname" validate:"omitempty,min=3"`
	PhoneNumber string `json:"phone" validate:"omitempty"`
	Role        string `json:"role" validate:"omitempty,oneof=user admin superadmin cashier staff"`
	NIK         string `json:"nik" validate:"omitempty"`
	DateOfBirth string `json:"date_of_birth" validate:"omitempty"`
}

type ResetPasswordRequest struct {
	NewPassword string `json:"new_password" validate:"required,min=8"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=6"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

type RefreshTokenData struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	FullName string `json:"fullname"`
	Role     string `json:"role"`
}
