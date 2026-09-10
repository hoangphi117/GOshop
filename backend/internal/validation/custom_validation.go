package validation

import (
	"fmt"
	"goshop/internal/utils"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func RegisterCustomValidation(v *validator.Validate) {

	var blockedDomains = map[string]bool{
		"blacklist.com": true,
		"edu.vn":        true,
		"abc.com":       true,
	}

	v.RegisterValidation("email_advanced", func(fl validator.FieldLevel) bool {
		email := fl.Field().String()

		parts := strings.Split(email, "@")
		if len(parts) != 2 {
			return false
		}

		domain := utils.NormalizeString(parts[1])

		return !blockedDomains[domain]
	})

	v.RegisterValidation("password_strong", func(fl validator.FieldLevel) bool {
		password := fl.Field().String()

		if len(password) < 8 {
			return false
		}

		hasLower := regexp.MustCompile(`[a-z]`).MatchString(password)
		hasUpper := regexp.MustCompile(`[A-Z]`).MatchString(password)
		hasDigit := regexp.MustCompile(`[0-9]`).MatchString(password)
		hasSpecial := regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?` + "`" + `~]`).MatchString(password)

		return hasLower && hasUpper && hasDigit && hasSpecial
	})

	var slugRegex = regexp.MustCompile(`^[a-z0-9]+(?:[-.][a-z0-9]+)*$`)
	v.RegisterValidation("slug", func(fl validator.FieldLevel) bool {
		return slugRegex.MatchString(fl.Field().String())
	})

	var searchRegex = regexp.MustCompile(`^[\p{L}\p{N}\s]+$`)
	v.RegisterValidation("search", func(fl validator.FieldLevel) bool {
		return searchRegex.MatchString(fl.Field().String())
	})

	v.RegisterValidation("min_float", func(fl validator.FieldLevel) bool {
		minStr := fl.Param()
		minVal, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return false
		}
		return fl.Field().Float() >= minVal
	})

	v.RegisterValidation("max_float", func(fl validator.FieldLevel) bool {
		minStr := fl.Param()
		maxVal, err := strconv.ParseFloat(minStr, 64)
		if err != nil {
			return false
		}
		return fl.Field().Float() <= maxVal
	})

	v.RegisterValidation("file_ext", func(fl validator.FieldLevel) bool {
		fileName := fl.Field().String()

		allowedStr := fl.Param()
		if allowedStr == "" {
			return false
		}

		allowedExt := strings.Fields(allowedStr)
		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(fileName)), ".")

		for _, allowed := range allowedExt {
			if ext == strings.ToLower(allowed) {
				return true
			}
		}
		return false
	})
}

func HandleValidationErrors(err error) gin.H {
	if validationError, ok := err.(validator.ValidationErrors); ok {
		errors := make(map[string]string)

		for _, e := range validationError {

			_, rawPath, _ := strings.Cut(e.Namespace(), ".")

			parts := strings.Split(rawPath, ".")

			for i, part := range parts {
				if strings.Contains(part, "[") {
					idx := strings.Index(part, "[")
					base := utils.CamelToSnake(part[:idx])
					index := part[idx:]
					parts[i] = base + index
				} else {
					parts[i] = utils.CamelToSnake(part)
				}
			}

			fieldPath := strings.Join(parts, ".")

			switch e.Tag() {
			case "gt":
				errors[fieldPath] = fmt.Sprintf("%s must be greater than %s", fieldPath, e.Param())
			case "gte":
				errors[fieldPath] = fmt.Sprintf("%s must be greater than or equal to %s", fieldPath, e.Param())
			case "lt":
				errors[fieldPath] = fmt.Sprintf("%s must be less than %s", fieldPath, e.Param())
			case "lte":
				errors[fieldPath] = fmt.Sprintf("%s must be less than or equal to %s", fieldPath, e.Param())
			case "gtefield":
				errors[fieldPath] = fmt.Sprintf("%s must be greater than or equal to %s", fieldPath, e.Param())
			case "ltefield":
				errors[fieldPath] = fmt.Sprintf("%s must be less than or equal to %s", fieldPath, e.Param())
			case "eqfield":
				errors[fieldPath] = fmt.Sprintf("%s must be equal to %s", fieldPath, e.Param())
			case "uuid":
				errors[fieldPath] = fieldPath + " must be a valid uuid"
			case "slug":
				errors[fieldPath] = fieldPath + " can only contain letters, numbers, hyphens, or periods"
			case "min":
				errors[fieldPath] = fmt.Sprintf("%s must be at least %s characters", fieldPath, e.Param())
			case "max":
				errors[fieldPath] = fmt.Sprintf("%s must be no more than %s", fieldPath, e.Param())
			case "min_float":
				errors[fieldPath] = fmt.Sprintf("%s must be greater than %s", fieldPath, e.Param())
			case "max_float":
				errors[fieldPath] = fmt.Sprintf("%s must be less than %s", fieldPath, e.Param())
			case "oneof":
				allowedValue := strings.Join(strings.Split(e.Param(), " "), " ")
				errors[fieldPath] = fmt.Sprintf("%s must be one of the following: %s", fieldPath, allowedValue)
			case "required":
				errors[fieldPath] = fmt.Sprintf("%s is required", fieldPath)
			case "search":
				errors[fieldPath] = fmt.Sprintf("%s can only contain lowercase letters, uppercase letters, numbers, and spaces", fieldPath)
			case "email":
				errors[fieldPath] = fmt.Sprintf("%s is not a valid email address", fieldPath)
			case "datetime":
				errors[fieldPath] = fmt.Sprintf("%s is not a valid date/time format (YYYY-MM-DD)", fieldPath)
			case "file_ext":
				allowedExt := strings.Join(strings.Split(e.Param(), " "), " ")
				errors[fieldPath] = fmt.Sprintf("%s does not contain an allowed file extension among %s", fieldPath, allowedExt)
			case "email_advanced":
				errors[fieldPath] = fmt.Sprintf("%s is in the list of blocked emails", fieldPath)
			case "password_strong":
				errors[fieldPath] = fmt.Sprintf("%s must contain at least 8 characters including lowercase, uppercase, numbers, and special characters", fieldPath)
			case "url":
				errors[fieldPath] = fmt.Sprintf("%s must be a valid URL", fieldPath)
			}
		}

		return gin.H{"error": errors}

	}

	return gin.H{"error": "Invalid request", "detail": err.Error()}
}
