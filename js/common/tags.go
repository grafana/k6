package common

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"go.k6.io/k6/metrics"
)

// ApplyCustomUserTags modifies the given metrics.TagsAndMeta object with the
// user specified custom tags and metadata (i.e. indexed or non-indexed tags).
// It expects to receive the `keyValues` object in the `{key1: value1, key2:
// value2, ...}` format.
func ApplyCustomUserTags(rt *goja.Runtime, tagsAndMeta *metrics.TagsAndMeta, keyValues goja.Value) error {
	if keyValues == nil || goja.IsNull(keyValues) || goja.IsUndefined(keyValues) {
		return nil
	}

	keyValuesObj := keyValues.ToObject(rt)

	for _, key := range keyValuesObj.Keys() {
		if err := ApplyCustomUserTag(rt, tagsAndMeta, key, keyValuesObj.Get(key)); err != nil {
			return err
		}
	}

	return nil
}

// ApplyCustomUserTag modifies the given metrics.TagsAndMeta object with the
// given custom tag and its value, either as an indexed tag or, if the value is
// an object in the `{key1: value1, key2: value2, ...}` format, as metadata.
func ApplyCustomUserTag(rt *goja.Runtime, tagsAndMeta *metrics.TagsAndMeta, key string, val goja.Value) error {
	kind := reflect.Invalid
	if typ := val.ExportType(); typ != nil {
		kind = typ.Kind()
	}

	switch kind {
	case
		reflect.String,
		reflect.Bool,
		reflect.Int64,
		reflect.Float64:

		tagsAndMeta.SetTag(key, val.String())

		return nil

	case reflect.Map:
		objVal := val.ToObject(rt)
		val = objVal.Get("value")
		indexed := objVal.Get("index")

		if val == nil || goja.IsUndefined(val) || goja.IsNull(val) ||
			indexed == nil || goja.IsUndefined(indexed) || goja.IsNull(indexed) {
			return fmt.Errorf(
				"setting '%s' metric tag failed; tag value objects need to have the 'value' and 'index' properties",
				key,
			)
		}

		if indexed.ToBoolean() {
			tagsAndMeta.SetTag(key, val.String())
		} else {
			tagsAndMeta.SetMetadata(key, val.String())
		}

		return nil

	default:
		return fmt.Errorf(
			"setting '%s' metric tag failed; only String, Boolean and Number types, or objects"+
				" with the 'value' and 'index' properties, are accepted as a metric tag values",
			key,
		)
	}
}
