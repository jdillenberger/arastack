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

		switch fv.Kind() {
		case reflect.Struct:
			walkStruct(fv, envKey)
			continue
		case reflect.Ptr:
			// Dereference pointer fields (e.g., *bool) and set the underlying value.
			val, ok := os.LookupEnv(envKey)
			if !ok {
				continue
			}
			elem := fv.Type().Elem()
			newVal := reflect.New(elem)
			setFieldFromString(newVal.Elem(), val)
			fv.Set(newVal)
			continue
		}

		// Slice of structs: scan for indexed env vars like PREFIX_0_FIELD, PREFIX_1_FIELD, ...
		if fv.Kind() == reflect.Slice && fv.Type().Elem().Kind() == reflect.Struct {
			elemType := fv.Type().Elem()
			var items []reflect.Value
			for idx := 0; ; idx++ {
				idxPrefix := envKey + "_" + strconv.Itoa(idx)
				elem := reflect.New(elemType).Elem()
				if !hasEnvWithPrefix(elem, idxPrefix) {
					break
				}
				walkStruct(elem, idxPrefix)
				items = append(items, elem)
			}
			if len(items) > 0 {
				sl := reflect.MakeSlice(fv.Type(), len(items), len(items))
				for i, item := range items {
					sl.Index(i).Set(item)
				}
				fv.Set(sl)
			}
			continue
		}

		val, ok := os.LookupEnv(envKey)
		if !ok {
			continue
		}

		setFieldFromString(fv, val)
	}
}

// hasEnvWithPrefix checks if any env var exists matching the struct fields under the given prefix.
func hasEnvWithPrefix(v reflect.Value, prefix string) bool {
	if v.Kind() != reflect.Struct {
		return false
	}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := field.Tag.Get("yaml")
		if tag == "" || tag == "-" {
			continue
		}
		if idx := strings.IndexByte(tag, ','); idx != -1 {
			tag = tag[:idx]
		}
		if tag == "" {
			continue
		}
		envKey := prefix + "_" + strings.ToUpper(tag)
		if _, ok := os.LookupEnv(envKey); ok {
			return true
		}
		// Check nested structs recursively.
		if field.Type.Kind() == reflect.Struct {
			if hasEnvWithPrefix(reflect.New(field.Type).Elem(), envKey) {
				return true
			}
		}
	}
	return false
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
