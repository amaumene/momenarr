package querystring

import (
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

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

		tag := fieldType.Tag.Get("url")
		if tag == "" || tag == "-" {
			continue
		}

		name := strings.Split(tag, ",")[0]
		if name == "" {
			name = fieldType.Name
		}

		if !field.IsValid() || field.IsZero() {
			continue
		}

		switch field.Kind() {
		case reflect.String:
			values.Set(name, field.String())
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			values.Set(name, strconv.FormatInt(field.Int(), 10))
		case reflect.Bool:
			values.Set(name, strconv.FormatBool(field.Bool()))
		}
	}

	return values, nil
}
