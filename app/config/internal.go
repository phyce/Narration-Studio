package config

import (
	"reflect"
	"strings"
)

func findFieldByJSONTag(dest reflect.Value, jsonTag string) (reflect.Value, bool) {
	destType := dest.Type()
	for i := 0; i < dest.NumField(); i++ {
		field := dest.Field(i)
		fieldType := destType.Field(i)
		tag := fieldType.Tag.Get("json")
		name := strings.SplitN(tag, ",", 2)[0]
		if name == "" || name == "-" {
			continue
		}
		if name == jsonTag {
			return field, true
		}
	}
	return reflect.Value{}, false
}
