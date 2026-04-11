package util

import (
	"errors"
	"regexp"

	"reflect"
	"strings"

	"github.com/go-playground/validator/v10"
	"github.com/faizalramadhan/pos-be/internal/application/dto"

	"github.com/go-playground/locales/en"
	ut "github.com/go-playground/universal-translator"

	translations "github.com/go-playground/validator/v10/translations/en"
)

// ValidateRequest performs validation on the provided data using the struct tags and validation rules.
// It takes a single parameter: `data` (any) representing the data to be validated.
// The function returns a slice of `entities.FieldError` containing any validation errors encountered during the validation process.
func ValidateRequest(data any) []dto.ApiFieldError {
	// Create a new language translator for the 'id' locale
	lang := en.New()
	uni := ut.New(lang, lang)

	// Get the translator for the 'id' locale
	trans, _ := uni.GetTranslator("en")

	// Create a new validator instance
	validate := validator.New()

	// Register default translations for the validator
	translations.RegisterDefaultTranslations(validate, trans)

	// Register a custom tag name function for the validator
	validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
		name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
		if name == "-" {
			return ""
		}
		return name
	})

	validate.RegisterValidation("phoneNumber", func(fl validator.FieldLevel) bool {
		phoneRegex := regexp.MustCompile(`^628(52|53|11|12|13|21|22|23|51)\d{6,13}$`)
		// phoneRegex := regexp.MustCompile(`^62\d{6,14}$`)

		return phoneRegex.MatchString(fl.Field().String())
	})

	// Perform the data validation
	err := validate.Struct(data)

	// Check if there are validation errors
	if errors.As(err, &validator.ValidationErrors{}) {
		var outs []dto.ApiFieldError
		for _, e := range err.(validator.ValidationErrors) {
			// Create a new FieldError instance for each validation error encountered
			outs = append(outs, dto.ApiFieldError{
				Field:   e.Field(),
				Message: e.Translate(trans),
			})
		}
		return outs
	}

	// Return nil if there are no validation errors
	return nil
}
