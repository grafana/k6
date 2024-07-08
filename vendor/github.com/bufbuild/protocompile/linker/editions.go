// Copyright 2020-2024 Buf Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package linker

import (
	"fmt"
	"sync"

	"google.golang.org/protobuf/encoding/prototext"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
)

var (
	editionDefaults     map[descriptorpb.Edition]*descriptorpb.FeatureSet
	editionDefaultsInit sync.Once

	featureSetDescriptor       = (*descriptorpb.FeatureSet)(nil).ProtoReflect().Descriptor()
	fieldPresenceField         = featureSetDescriptor.Fields().ByName("field_presence")
	repeatedFieldEncodingField = featureSetDescriptor.Fields().ByName("repeated_field_encoding")
	messageEncodingField       = featureSetDescriptor.Fields().ByName("message_encoding")
	enumTypeField              = featureSetDescriptor.Fields().ByName("enum_type")
	jsonFormatField            = featureSetDescriptor.Fields().ByName("json_format")
)

type hasFeatures interface {
	GetFeatures() *descriptorpb.FeatureSet
}

var _ hasFeatures = (*descriptorpb.FileOptions)(nil)
var _ hasFeatures = (*descriptorpb.MessageOptions)(nil)
var _ hasFeatures = (*descriptorpb.FieldOptions)(nil)
var _ hasFeatures = (*descriptorpb.OneofOptions)(nil)
var _ hasFeatures = (*descriptorpb.ExtensionRangeOptions)(nil)
var _ hasFeatures = (*descriptorpb.EnumOptions)(nil)
var _ hasFeatures = (*descriptorpb.EnumValueOptions)(nil)
var _ hasFeatures = (*descriptorpb.ServiceOptions)(nil)
var _ hasFeatures = (*descriptorpb.MethodOptions)(nil)

// resolveFeature resolves a feature for the given descriptor. It uses the function
// to extract a value from a feature set. The function should return false as the
// second argument if the first value it returns was not actually present in the
// feature set.
func resolveFeature(d protoreflect.Descriptor, field protoreflect.FieldDescriptor) protoreflect.Value {
	edition := getEdition(d)
	if edition == descriptorpb.Edition_EDITION_PROTO2 || edition == descriptorpb.Edition_EDITION_PROTO3 {
		// these syntax levels can't specify features, so we can short-circuit the search
		// through the descriptor hierarchy for feature overrides
		defaults := getEditionDefaults(edition)
		return defaults.ProtoReflect().Get(field) // returns default value if field is not present
	}
	for {
		var features *descriptorpb.FeatureSet
		if withFeatures, ok := d.Options().(hasFeatures); ok {
			// It should not be possible for 'ok' to ever be false...
			features = withFeatures.GetFeatures()
		}
		featuresRef := features.ProtoReflect()
		if featuresRef.Has(field) {
			return featuresRef.Get(field)
		}
		parent := d.Parent()
		if parent == nil {
			// We've reached the end of the inheritance chain. So we fall back to a default.
			defaults := getEditionDefaults(edition)
			return defaults.ProtoReflect().Get(field)
		}
		d = parent
	}
}

type hasEdition interface {
	// Edition returns the numeric value of a google.protobuf.Edition enum
	// value that corresponds to the edition of this file. If the file does
	// not use editions, it should return the enum value that corresponds
	// to the syntax level, EDITION_PROTO2 or EDITION_PROTO3.
	Edition() int32
}

var _ hasEdition = (*result)(nil)

func getEdition(d protoreflect.Descriptor) descriptorpb.Edition {
	withEdition, ok := d.ParentFile().(hasEdition)
	if !ok {
		// The parent file should always be a *result, so we should
		// never be able to actually get in here. If we somehow did
		// have another implementation of protoreflect.FileDescriptor,
		// it doesn't provide a way to get the edition, other than the
		// potentially expensive step of generating a FileDescriptorProto
		// and then querying for the edition from that. :/
		return descriptorpb.Edition_EDITION_UNKNOWN
	}
	return descriptorpb.Edition(withEdition.Edition())
}

func getEditionDefaults(edition descriptorpb.Edition) *descriptorpb.FeatureSet {
	editionDefaultsInit.Do(func() {
		editionDefaults = make(map[descriptorpb.Edition]*descriptorpb.FeatureSet, len(descriptorpb.Edition_name))
		// Compute default for all known editions in descriptorpb.
		for editionInt := range descriptorpb.Edition_name {
			edition := descriptorpb.Edition(editionInt)
			defaults := &descriptorpb.FeatureSet{}
			defaultsRef := defaults.ProtoReflect()
			fields := defaultsRef.Descriptor().Fields()
			// Note: we are not computing defaults for extensions. Those are not needed
			// by anything in the compiler, so we can get away with just computing
			// defaults for these static, non-extension fields.
			for i, length := 0, fields.Len(); i < length; i++ {
				field := fields.Get(i)
				opts, ok := field.Options().(*descriptorpb.FieldOptions)
				if !ok {
					continue // this is most likely impossible
				}
				maxEdition := descriptorpb.Edition(-1)
				var maxVal string
				for _, def := range opts.EditionDefaults {
					if def.GetEdition() <= edition && def.GetEdition() > maxEdition {
						maxEdition = def.GetEdition()
						maxVal = def.GetValue()
					}
				}
				if maxEdition == -1 {
					// no matching default found
					continue
				}
				// We use a typed nil so that it won't fall back to the global registry. Features
				// should not use extensions or google.protobuf.Any, so a nil *Types is fine.
				unmarshaler := prototext.UnmarshalOptions{Resolver: (*protoregistry.Types)(nil)}
				// The string value is in the text format: either a field value literal or a
				// message literal. (Repeated and map features aren't supported, so there's no
				// array or map literal syntax to worry about.)
				if field.Kind() == protoreflect.MessageKind || field.Kind() == protoreflect.GroupKind {
					fldVal := defaultsRef.NewField(field)
					err := unmarshaler.Unmarshal([]byte(maxVal), fldVal.Message().Interface())
					if err != nil {
						continue // should we fail somehow??
					}
					defaultsRef.Set(field, fldVal)
					continue
				}
				// The value is the textformat for the field. But prototext doesn't provide a way
				// to unmarshal a single field value. To work around, we unmarshal into the enclosing
				// message, so we prefix the value with the field name.
				maxVal = fmt.Sprintf("%s: %s", field.Name(), maxVal)
				// Sadly, prototext doesn't support a Merge option. So we can't merge the value
				// directly into the supplied msg. We have to instead unmarshal into an empty
				// message and then use that to set the field in the supplied msg. :(
				empty := defaultsRef.Type().New()
				err := unmarshaler.Unmarshal([]byte(maxVal), empty.Interface())
				if err != nil {
					continue // should we fail somehow??
				}
				defaultsRef.Set(field, empty.Get(field))
			}
			editionDefaults[edition] = defaults
		}
	})
	return editionDefaults[edition]
}

func isJSONCompliant(d protoreflect.Descriptor) bool {
	jsonFormat := resolveFeature(d, jsonFormatField)
	return descriptorpb.FeatureSet_JsonFormat(jsonFormat.Enum()) == descriptorpb.FeatureSet_ALLOW
}
