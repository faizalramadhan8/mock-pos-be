package middleware

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/faizalramadhan/pos-be/internal/application/dto"
	"github.com/faizalramadhan/pos-be/internal/domain/enum"
	"github.com/faizalramadhan/pos-be/pkg/util"
)

type RBACMiddleware struct {
	Secret   string
	Duration time.Duration
}

func NewRBACMiddleware(secret string, duration time.Duration) *RBACMiddleware {
	return &RBACMiddleware{
		Secret:   secret,
		Duration: duration,
	}
}

// allowRole is a middleware function that validates and refreshes JWT tokens.
// It checks the "Authorization" header in the request, validates the JWT token,
// and refreshes the token if it is about to expire.
// It returns a Fiber handler function that can be used as middleware.
func (m RBACMiddleware) allowRole(allowed []string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get the "Authorization" header from the request
		authorization := c.Get(fiber.HeaderAuthorization)

		// Split the authorization header into fields
		authFields := strings.Fields(authorization)
		if len(authFields) < 2 {
			// Return unauthorized response if the authorization header is missing or incomplete
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ApiResponse{
				Code:    fiber.ErrUnauthorized.Code,
				Message: fiber.ErrUnauthorized.Message,
				Error:   jwt.ErrTokenSignatureInvalid.Error(),
			})
		}

		if authFields[0] != "Bearer" {
			// Return unauthorized response if the authorization type is not "Bearer"
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ApiResponse{
				Code:    fiber.ErrUnauthorized.Code,
				Message: fiber.ErrUnauthorized.Message,
				Error:   jwt.ErrTokenSignatureInvalid.Error(),
			})
		}

		// Unmarshal and validate the JWT claims using the provided secret
		claims, err := util.UnmarshalClaims(m.Secret, authFields[1])
		if err != nil {
			// Return unauthorized response if the token is invalid or expired
			return c.Status(fiber.StatusUnauthorized).JSON(dto.ApiResponse{
				Code:    fiber.StatusUnauthorized,
				Message: err.Error(),
			})
		}

		// Refresh the token if it is about to expire
		if time.Until(claims.ExpiresAt.Time) <= 10*time.Minute {
			claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(m.Duration))
			token, err := util.MarshalClaims(m.Secret, claims)
			if err != nil {
				return c.Status(fiber.StatusUnauthorized).JSON(dto.ApiResponse{
					Code:    fiber.StatusUnauthorized,
					Message: err.Error(),
				})
			}
			authorization = token.GetBearer()
		}

		// Set the updated authorization header to include the refreshed token
		c.Set(fiber.HeaderAuthorization, authorization)

		// Store the validated JWT claims in the context locals for future use
		c.Locals("session", claims)
		// Return forbidden response if the user's role does not match the required role

		for _, allow := range allowed {

			roles := strings.Split(claims.Role, ",")

			for _, role := range roles {

				if allow == role {
					return c.Next()
				}
			}
		}

		// Return StatusUnauthorized if role user not in list allowd
		return c.Status(fiber.StatusForbidden).JSON(dto.ApiResponse{
			Code:    fiber.ErrUnauthorized.Code,
			Message: fiber.ErrUnauthorized.Message,
			Error:   enum.ErrAccessForbidden.Error(),
		})
	}
}

func (m RBACMiddleware) AllowAdmins() fiber.Handler {
	return m.allowRole([]string{string(enum.RoleAdmin), string(enum.RoleSuperAdmin)})
}

func (m RBACMiddleware) AllowSuperAdmin() fiber.Handler {
	return m.allowRole([]string{string(enum.RoleSuperAdmin)})
}

func (m RBACMiddleware) AllowAll() fiber.Handler {
	return m.allowRole([]string{
		string(enum.RoleUser),
		string(enum.RoleAdmin),
		string(enum.RoleSuperAdmin),
		string(enum.RoleCashier),
		string(enum.RoleStaff),
	})
}

func (m RBACMiddleware) AllowInventoryWrite() fiber.Handler {
	return m.allowRole([]string{
		string(enum.RoleSuperAdmin),
		string(enum.RoleAdmin),
		string(enum.RoleStaff),
	})
}

func (m RBACMiddleware) AllowCashier() fiber.Handler {
	return m.allowRole([]string{
		string(enum.RoleSuperAdmin),
		string(enum.RoleAdmin),
		string(enum.RoleCashier),
	})
}
