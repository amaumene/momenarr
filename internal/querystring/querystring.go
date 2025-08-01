// Package querystring provides URL query string encoding functionality for structs.
package querystring

import (
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

// Values encodes a struct into URL query parameters.
func Values(v interface{}) (url.Values, error) {
	values := url.Values{}
	rv := reflect.ValueOf(v)

	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}

	if rv.Kind() != reflect.Struct {
		return values, nil
	}

	rt := rv.Type()
	for i := 0; i < rv.NumField(); i++ {
		field := rv.Field(i)
		fieldType := rt.Field(i)

		name := getFieldName(fieldType)
		if name == "" {
			continue
		}

		if !field.IsValid() || field.IsZero() {
			continue
		}

		encodeField(values, name, field)
	}

	return values, nil
}

// getFieldName extracts the field name from struct tag
func getFieldName(field reflect.StructField) string {
	tag := field.Tag.Get("url")
	if tag == "" || tag == "-" {
		return ""
	}

	name := strings.Split(tag, ",")[0]
	if name == "" {
		name = field.Name
	}

	return name
}

// encodeField encodes a field value into URL values
func encodeField(values url.Values, name string, field reflect.Value) {
	switch field.Kind() {
	case reflect.String:
		values.Set(name, field.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		values.Set(name, strconv.FormatInt(field.Int(), 10))
	case reflect.Bool:
		values.Set(name, strconv.FormatBool(field.Bool()))
	}
}
