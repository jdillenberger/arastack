package config

import (
	"os"
	"reflect"
	"strconv"
	"strings"
)

// applyEnvOverrides walks cfg recursively using yaml struct tags and applies
// matching environment variables. For example, with prefix "ARAALERT", a field
// tagged yaml:"server" containing a field tagged yaml:"port" maps to ARAALERT_SERVER_PORT.
func applyEnvOverrides(cfg any, prefix string) {
	if prefix == "" {
		return
	}
	v := reflect.ValueOf(cfg)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	walkStruct(v, prefix)
}

func walkStruct(v reflect.Value, prefix string) {
	if v.Kind() != reflect.Struct {
		return
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fv := v.Field(i)

		if !fv.CanSet() {
			continue
		}

		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		// Strip yaml options like ",omitempty"
		if idx := strings.IndexByte(tag, ','); idx != -1 {
			tag = tag[:idx]
		}
		if tag == "" {
			continue
		}

		envKey := prefix + "_" + strings.ToUpper(tag)

		if fv.Kind() == reflect.Struct {
			walkStruct(fv, envKey)
			continue
		}

		val, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		setFieldFromString(fv, val)
	}
}

func setFieldFromString(fv reflect.Value, val string) {
	switch fv.Kind() {
	case reflect.String:
		fv.SetString(val)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n, err := strconv.ParseInt(val, 10, 64); err == nil {
			fv.SetInt(n)
		}
	case reflect.Bool:
		if b, err := strconv.ParseBool(val); err == nil {
			fv.SetBool(b)
		}
	case reflect.Slice:
		if fv.Type().Elem().Kind() == reflect.String {
			parts := strings.Split(val, ",")
			for i := range parts {
				parts[i] = strings.TrimSpace(parts[i])
			}
			fv.Set(reflect.ValueOf(parts))
		}
	}
}
