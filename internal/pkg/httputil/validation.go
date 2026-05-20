package httputil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

func BindJSON(c *gin.Context, dst any) bool {
	if err := c.ShouldBindJSON(dst); err != nil {
		code, message := BindingError(err, dst)
		c.JSON(400, gin.H{"error": code, "message": message})
		return false
	}
	return true
}

func BindingError(err error, dst any) (string, string) {
	var validationErrors validator.ValidationErrors
	if errors.As(err, &validationErrors) && len(validationErrors) > 0 {
		fieldName := jsonFieldName(dst, validationErrors[0].Field())
		return validationErrorCode(fieldName, validationErrors[0].Tag()), validationErrorMessage(fieldName, validationErrors[0])
	}

	var syntaxError *json.SyntaxError
	if errors.As(err, &syntaxError) || errors.Is(err, io.ErrUnexpectedEOF) {
		return "INVALID_JSON", "Request body must be valid JSON"
	}

	var typeError *json.UnmarshalTypeError
	if errors.As(err, &typeError) {
		fieldName := typeError.Field
		if fieldName == "" {
			fieldName = "request"
		}
		return validationErrorCode(fieldName, "type"), fmt.Sprintf("%s has an invalid type", fieldName)
	}

	return "INVALID_REQUEST", "Request body is invalid"
}

func jsonFieldName(dst any, structField string) string {
	typ := reflect.TypeOf(dst)
	for typ != nil && typ.Kind() == reflect.Pointer {
		typ = typ.Elem()
	}
	if typ == nil || typ.Kind() != reflect.Struct {
		return structField
	}

	field, ok := typ.FieldByName(structField)
	if !ok {
		return structField
	}
	tag := strings.Split(field.Tag.Get("json"), ",")[0]
	if tag == "" || tag == "-" {
		return structField
	}
	return tag
}

func validationErrorCode(fieldName, tag string) string {
	base := upperSnake(fieldName)
	switch tag {
	case "required":
		return base + "_REQUIRED"
	case "min", "max", "gt", "gte", "lt", "lte", "len", "numeric", "oneof", "type":
		return "INVALID_" + base
	default:
		return "INVALID_REQUEST"
	}
}

func validationErrorMessage(fieldName string, fieldErr validator.FieldError) string {
	switch fieldErr.Tag() {
	case "required":
		return fieldName + " is required"
	case "min":
		return fmt.Sprintf("%s must be at least %s", fieldName, fieldErr.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s", fieldName, fieldErr.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", fieldName, fieldErr.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", fieldName, fieldErr.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", fieldName, fieldErr.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", fieldName, fieldErr.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", fieldName, fieldErr.Param())
	case "numeric":
		return fieldName + " must contain only digits"
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", fieldName, fieldErr.Param())
	default:
		return fieldName + " is invalid"
	}
}

func upperSnake(value string) string {
	var builder strings.Builder
	for i, r := range value {
		if r == '-' || r == ' ' {
			builder.WriteByte('_')
			continue
		}
		if unicode.IsUpper(r) && i > 0 {
			builder.WriteByte('_')
		}
		builder.WriteRune(unicode.ToUpper(r))
	}
	return builder.String()
}
