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

	var searchRegex = regexp.MustCompile(`^[a-zA-Z0-9\s]+$`)
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
				errors[fieldPath] = fmt.Sprintf("%s phải lớn hơn %s", fieldPath, e.Param())
			case "gte":
				errors[fieldPath] = fmt.Sprintf("%s phải lớn hơn hoặc bằng %s", fieldPath, e.Param())
			case "lt":
				errors[fieldPath] = fmt.Sprintf("%s phải bé hơn %s", fieldPath, e.Param())
			case "lte":
				errors[fieldPath] = fmt.Sprintf("%s phải bé hơn hoặc bằng %s", fieldPath, e.Param())
			case "uuid":
				errors[fieldPath] = fieldPath + " phải là uuid hợp lệ"
			case "slug":
				errors[fieldPath] = fieldPath + " chỉ được chứa chữ, số, dấu gạch ngang hoặc dấu chấm"
			case "min":
				errors[fieldPath] = fmt.Sprintf("%s phải lớn hơn %s kí tự", fieldPath, e.Param())
			case "max":
				errors[fieldPath] = fmt.Sprintf("%s phải ít hơn %s kí tự", fieldPath, e.Param())
			case "min_float":
				errors[fieldPath] = fmt.Sprintf("%s phải lớn hơn %s", fieldPath, e.Param())
			case "max_float":
				errors[fieldPath] = fmt.Sprintf("%s phải nhỏ hơn %s", fieldPath, e.Param())
			case "oneof":
				allowedValue := strings.Join(strings.Split(e.Param(), " "), " ")
				errors[fieldPath] = fmt.Sprintf("%s phải là một trong các %s", fieldPath, allowedValue)
			case "required":
				errors[fieldPath] = fmt.Sprintf("%s là bắt buộc", fieldPath)
			case "search":
				errors[fieldPath] = fmt.Sprintf("%s chỉ được chứa chữ thường, in hoa, số và khoảng trắng", fieldPath)
			case "email":
				errors[fieldPath] = fmt.Sprintf("%s không đúng định dạng", fieldPath)
			case "datetime":
				errors[fieldPath] = fmt.Sprintf("%s không đúng định dạng YYYY-MM-DD", fieldPath)
			case "file_ext":
				allowedExt := strings.Join(strings.Split(e.Param(), " "), " ")
				errors[fieldPath] = fmt.Sprintf("%s không chứa đuôi file cho phép trong %s", fieldPath, allowedExt)
			case "email_advanced":
				errors[fieldPath] = fmt.Sprintf("%s nằm trong danh sách bị cấm", fieldPath)
			case "password_strong":
				errors[fieldPath] = fmt.Sprintf("%s phải chứa ít nhất 8 kí tự gồm (chữ thường, in hoa, số và kí tự đặc biệt)", fieldPath)
			}
		}

		return gin.H{"error": errors}

	}

	return gin.H{"error": "Yeu cau khong hop le", "detail": err.Error()}
}
