package env

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
)

func ParseAs[T any]() (T, error) {
	var out T
	v := reflect.ValueOf(&out).Elem()
	if v.Kind() != reflect.Struct {
		return out, fmt.Errorf("env: ParseAs expects a struct type")
	}
	t := v.Type()
	for idx := 0; idx < t.NumField(); idx++ {
		field := t.Field(idx)
		if field.PkgPath != "" {
			continue
		}
		name, required, defaultValue := parseTag(field.Tag.Get("env"), field.Tag.Get("envDefault"))
		if name == "" {
			continue
		}
		raw, ok := os.LookupEnv(name)
		if !ok || strings.TrimSpace(raw) == "" {
			raw = defaultValue
		}
		if strings.TrimSpace(raw) == "" && required {
			return out, fmt.Errorf("env: missing required variable %s", name)
		}
		if raw == "" {
			continue
		}
		if err := setValue(v.Field(idx), raw); err != nil {
			return out, fmt.Errorf("env: %s: %w", name, err)
		}
	}
	return out, nil
}

func parseTag(tag, defaultValue string) (name string, required bool, fallback string) {
	parts := strings.Split(tag, ",")
	if len(parts) > 0 {
		name = strings.TrimSpace(parts[0])
	}
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "required" {
			required = true
		}
	}
	return name, required, defaultValue
}

func setValue(field reflect.Value, raw string) error {
	if !field.CanSet() {
		return nil
	}
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			field.Set(reflect.New(field.Type().Elem()))
		}
		field = field.Elem()
	}
	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return err
		}
		field.SetInt(value)
	case reflect.Bool:
		value, err := strconv.ParseBool(raw)
		if err != nil {
			return err
		}
		field.SetBool(value)
	default:
		return fmt.Errorf("unsupported field type %s", field.Type())
	}
	return nil
}
