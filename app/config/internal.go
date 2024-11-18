package config

import (
	"fmt"
	"nstudio/app/common/issue"
	"reflect"
)

func updateStruct(destination, source reflect.Value) error {
	if source.Kind() == reflect.Ptr {
		source = source.Elem()
	}

	if source.Kind() != reflect.Struct {
		return issue.Trace(fmt.Errorf("expecting a struct or a pointer to a struct in source"))
	}

	for i := 0; i < source.NumField(); i++ {
		sourceField := source.Field(i)
		sourceFieldType := source.Type().Field(i)

		jsonTag := sourceFieldType.Tag.Get("json")
		if jsonTag == "" {
			jsonTag = sourceFieldType.Name
		}

		destinationField, found := findFieldByJSONTag(destination, jsonTag)
		if !found {
			continue
		}

		if sourceField.Kind() == reflect.Struct && destinationField.Kind() == reflect.Struct {
			err := updateStruct(destinationField, sourceField)
			if err != nil {
				return err
			}
		} else {
			if !isZeroValue(sourceField) {
				if destinationField.CanSet() {
					destinationField.Set(sourceField)
				} else {
					return fmt.Errorf("cannot set field %s", destinationField.Type().Name())
				}
			}
		}
	}
	return nil
}

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

func isZeroValue(v reflect.Value) bool {
	zero := reflect.Zero(v.Type())
	return reflect.DeepEqual(v.Interface(), zero.Interface())
}
