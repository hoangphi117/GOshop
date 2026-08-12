package utils

import (
	"regexp"
	"strings"
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

func ConvertToInt32Pointer(value int) *int32 {
	if value == 0 {
		return nil
	}
	int32Value := int32(value)
	return &int32Value
}

func CapitalizeFirstLetter(str string) string {
	if len(str) == 0 {
		return str
	}
	return strings.ToUpper(string(str[0])) + str[1:]
}
