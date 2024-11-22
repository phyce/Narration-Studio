package config

import (
	"reflect"
)

func findFieldByJSONTag(dest reflect.Value, jsonTag string) (reflect.Value, bool) {
	destType := dest.Type()
	for i := 0; i < dest.NumField(); i++ {
		field := dest.Field(i)
		fieldType := destType.Field(i)
		tag := fieldType.Tag.Get("json")
		if tag == jsonTag {
			return field, true
		}
	}
	return reflect.Value{}, false
}
