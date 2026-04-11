package enum

import "errors"

var (
	ErrInternalServor  = errors.New("internal server error")
	ErrBadRequest      = errors.New("bad request")
	ErrAccessForbidden = errors.New("you are not authorized to access this URL")
	ErrUnauthorized    = errors.New("unauthorized")
	ErrNotFound        = errors.New("requested data is not found")

	// Token & Auth
	ErrInvalidToken             = errors.New("token is invalid")
	ErrExpiredToken             = errors.New("token has expired")
	ErrInvalidRefreshToken      = errors.New("refresh token is invalid")
	ErrIncorrectCredential      = errors.New("login failed. Email or password is incorrect")
	ErrEmailOrPasswordMissMatch = errors.New("email/password mismatch")

	// Parameter errors
	ErrBadParamInput    = errors.New("requested parameters are not valid")
	ErrMissingParameter = errors.New("missing parameter")
	ErrInvalidParameter = errors.New("invalid parameter")
)
