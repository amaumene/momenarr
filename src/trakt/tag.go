package trakt

import (
	"reflect"
	"strings"
)

// a straight copy from the "encoding/json" package. This is so we can
// parse a struct tag and apply some of the options JSON Marshal applies
// when adding dynamic fields to a JSON structure.

// marshalToMap helper function to convert a struct / pointer to a struct
// to a map.
// this is not recursive and only does a top level transformation.
func marshalToMap(i interface{}) map[string]interface{} {
	var m = make(map[string]interface{})
	if i == nil {
		return m
	}

	vo := reflect.ValueOf(i)
	if !vo.IsValid() {
		return m
	}

	if vo.Kind() == reflect.Ptr {
		return marshalToMap(vo.Elem().Interface())
	}

	if vo.Kind() != reflect.Struct {
		return m
	}

	v := reflect.TypeOf(i)

	for idx := 0; idx < v.NumField(); idx++ {
		vf := vo.Field(idx)
		if !vf.IsValid() {
			continue
		}

		fld := v.Field(idx)
		st, ok := fld.Tag.Lookup("json")
		if !ok || st == "-" {
			continue
		}

		tn, opt := parseTag(st)
		if opt.Contains("omitempty") && vf.IsZero() {
			continue
		}

		m[tn] = vf.Interface()
	}

	return m
}

// tagOptions is the string following a comma in a struct field's "json"
// tag, or the empty string. It does not include the leading comma.
type tagOptions string

// parseTag splits a struct field's json tag into its name and
// comma-separated options.
func parseTag(tag string) (string, tagOptions) {
	if idx := strings.Index(tag, ","); idx != -1 {
		return tag[:idx], tagOptions(tag[idx+1:])
	}
	return tag, tagOptions("")
}

// Contains reports whether a comma-separated list of options
// contains a particular substr flag. substr must be surrounded by a
// string boundary or commas.
func (o tagOptions) Contains(optionName string) bool {
	if len(o) == 0 {
		return false
	}
	s := string(o)
	for s != "" {
		var next string
		i := strings.Index(s, ",")
		if i >= 0 {
			s, next = s[:i], s[i+1:]
		}
		if s == optionName {
			return true
		}
		s = next
	}
	return false
}
