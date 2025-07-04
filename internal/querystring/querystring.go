package querystring

import (
	"fmt"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// Values encodes a struct into URL query string values
func Values(v interface{}) (url.Values, error) {
	values := url.Values{}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	
	if rv.Kind() != reflect.Struct {
		return nil, fmt.Errorf("querystring: Values() expects struct input, got %v", rv.Kind())
	}
	
	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)
		
		// Get the URL tag
		tag := fieldType.Tag.Get("url")
		if tag == "" || tag == "-" {
			continue
		}
		
		// Handle omitempty
		parts := strings.Split(tag, ",")
		name := parts[0]
		omitempty := false
		if len(parts) > 1 && parts[1] == "omitempty" {
			omitempty = true
		}
		
		// Skip zero values if omitempty
		if omitempty && isZero(field) {
			continue
		}
		
		// Encode the value
		if err := encodeValue(values, name, field); err != nil {
			return nil, err
		}
	}
	
	return values, nil
}

func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr, reflect.Slice, reflect.Map:
		return v.IsNil()
	}
	return false
}

func encodeValue(values url.Values, name string, v reflect.Value) error {
	switch v.Kind() {
	case reflect.String:
		values.Set(name, v.String())
	case reflect.Bool:
		values.Set(name, strconv.FormatBool(v.Bool()))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		values.Set(name, strconv.FormatInt(v.Int(), 10))
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		values.Set(name, strconv.FormatUint(v.Uint(), 10))
	case reflect.Float32, reflect.Float64:
		values.Set(name, strconv.FormatFloat(v.Float(), 'f', -1, 64))
	case reflect.Ptr:
		if !v.IsNil() {
			return encodeValue(values, name, v.Elem())
		}
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			if err := encodeValue(values, name, v.Index(i)); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("querystring: unsupported type %v for field %s", v.Kind(), name)
	}
	return nil
}