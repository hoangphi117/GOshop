package utils

import (
	"math"
	"math/big"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgtype"
)

var (
	matchFirstCap = regexp.MustCompile("(.)[A-Z][a-z]+")
	matchAllCap   = regexp.MustCompile("([a-z0-9])([A-Z])")
)

func CamelToSnake(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(str, "${1}_${2}")
	return strings.ToLower(snake)
}

func NormalizeString(str string) string {
	return strings.ToLower(strings.TrimSpace(str))
}

func ConvertToInt32Pointer(value int32) *int32 {
	if value == 0 {
		return nil
	}
	return &value
}

func CapitalizeFirstLetter(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToUpper(string(str[0])) + str[1:]
}

func FloatToNumeric(f float64) pgtype.Numeric {
	if math.IsNaN(f) {
		panic("FloatToNumeric: cannot convert NaN to numeric")
	}
	if math.IsInf(f, 0) {
		panic("FloatToNumeric: cannot convert infinite value to numeric")
	}

	s := strconv.FormatFloat(f, 'f', -1, 64)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := ""
	if len(parts) > 1 {
		decPart = parts[1]
	}

	combined := intPart + decPart
	exp := -len(decPart)

	combined = strings.TrimLeft(combined, "0")
	if combined == "" {
		combined = "0"
	}
	if combined == "-" || combined == "-0" {
		combined = "0"
	}

	intVal := new(big.Int)
	if _, ok := intVal.SetString(combined, 10); !ok {
		panic("FloatToNumeric: failed to parse numeric value from " + combined)
	}

	return pgtype.Numeric{Int: intVal, Exp: int32(exp), Valid: true}
}

func NumericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid || n.Int == nil {
		return 0
	}

	value := new(big.Float).SetInt(n.Int)
	if n.Exp < 0 {
		divisor := new(big.Float).SetFloat64(1)
		for i := int32(0); i < -n.Exp; i++ {
			divisor.Mul(divisor, big.NewFloat(10))
		}
		value.Quo(value, divisor)
	}
	if n.Exp > 0 {
		multiplier := new(big.Float).SetFloat64(1)
		for i := int32(0); i < n.Exp; i++ {
			multiplier.Mul(multiplier, big.NewFloat(10))
		}
		value.Mul(value, multiplier)
	}

	f, _ := value.Float64()
	return f
}

func RoundFloat(val float64, precision int) float64 {
	ratio := math.Pow(10, float64(precision))
	return math.Round(val*ratio) / ratio
}

func Int32Value(ptr *int32) int32 {
	if ptr == nil {
		return 0
	}
	return *ptr
}

func BoolValue(ptr *bool) bool {
	if ptr == nil {
		return false
	}
	return *ptr
}

func StringToPointer(s string) *string {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func FloatPointerToNumeric(f *float64) pgtype.Numeric {
	if f == nil {
		return pgtype.Numeric{Valid: false}
	}
	return FloatToNumeric(*f)
}
