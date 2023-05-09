package common

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"go.k6.io/k6/metrics"
)

// ApplyCustomUserTags modifies the given metrics.TagsAndMeta object with the
// user specified custom tags and metadata.
// It expects to receive the `keyValues` object in the `{key1: value1, key2:
// value2, ...}` format.
func ApplyCustomUserTags(rt *goja.Runtime, tagsAndMeta *metrics.TagsAndMeta, keyValues goja.Value) error {
	if keyValues == nil || goja.IsNull(keyValues) || goja.IsUndefined(keyValues) {
		return nil
	}

	keyValuesObj := keyValues.ToObject(rt)

	for _, key := range keyValuesObj.Keys() {
		if err := ApplyCustomUserTag(tagsAndMeta, key, keyValuesObj.Get(key)); err != nil {
			return err
		}
	}

	return nil
}

// ApplyCustomUserTag modifies the given metrics.TagsAndMeta object with the
// given custom tag and theirs value.
func ApplyCustomUserTag(tagsAndMeta *metrics.TagsAndMeta, key string, val goja.Value) error {
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

	default:
		return fmt.Errorf(
			"invalid value for metric tag '%s': "+
				"only String, Boolean and Number types are accepted as a metric tag values",
			key,
		)
	}
}

// ApplyCustomUserMetadata modifies the given metrics.TagsAndMeta object with the
// given custom metadata and their value.
func ApplyCustomUserMetadata(tagsAndMeta *metrics.TagsAndMeta, key string, val goja.Value) error {
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

		tagsAndMeta.SetMetadata(key, val.String())
		return nil

	default:
		return fmt.Errorf(
			"invalid value for metric metadata '%s': "+
				"only String, Boolean and Number types are accepted as a metric metadata value",
			key,
		)
	}
}
