package common

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"go.k6.io/k6/metrics"
)

// ApplyCustomUserTags modifies the given metrics.TagSet object with the
// user specified custom tags.
// It expects to receive the `keyValues` object in the `{key1: value1, key2:
// value2, ...}` format.
func ApplyCustomUserTags(rt *goja.Runtime, tags *metrics.TagSet, keyValues goja.Value) (*metrics.TagSet, error) {
	if keyValues == nil || goja.IsNull(keyValues) || goja.IsUndefined(keyValues) {
		return tags, nil
	}

	keyValuesObj := keyValues.ToObject(rt)

	newTags := tags
	var err error
	for _, key := range keyValuesObj.Keys() {
		if newTags, err = ApplyCustomUserTag(rt, newTags, key, keyValuesObj.Get(key)); err != nil {
			return nil, err
		}
	}

	return newTags, nil
}

// ApplyCustomUserTag modifies the given metrics.TagSet object with the
// given custom tag and its value.
func ApplyCustomUserTag(rt *goja.Runtime, tags *metrics.TagSet, key string, val goja.Value) (*metrics.TagSet, error) {
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

		return tags.With(key, val.String()), nil

	default:
		return nil, fmt.Errorf(
			"invalid value for metric tag '%s': "+
				"only String, Boolean and Number types are accepted as a metric tag values",
			key,
		)
	}
}
