package main

import (
	"fmt"
	"time"

	"github.com/iancoleman/strcase"
	terraformpb "github.com/protomesh/protoc-gen-terraform/proto/terraform"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/known/structpb"
)

type fieldInfo struct {
	fInfo *fileInfo

	value  *protogen.Field
	schema *terraformpb.FieldSchema

	fieldName string
	fieldKey  string

	okVar    string
	valueVar string
}

func newFieldInfo(fInfo *fileInfo, value *protogen.Field) *fieldInfo {

	name := string(value.Desc.Name())
	varName := strcase.ToCamel(name)

	return &fieldInfo{
		fInfo: fInfo,

		value:  value,
		schema: getFieldSchema(value.Desc),

		fieldName: strcase.ToCamel(name),
		fieldKey:  string(value.Desc.Name()),

		okVar:    fmt.Sprintf("ok%s", varName),
		valueVar: fmt.Sprintf("value%s", varName),
	}
}

func getFieldSchema(desc protoreflect.FieldDescriptor) *terraformpb.FieldSchema {

	opts, ok := desc.Options().(*descriptorpb.FieldOptions)
	if !ok {
		panic("Invalid message options")
	}

	if opts != nil && proto.HasExtension(opts, terraformpb.E_FieldSchema) {

		return proto.GetExtension(opts, terraformpb.E_FieldSchema).(*terraformpb.FieldSchema)

	}

	return &terraformpb.FieldSchema{
		IsTypeSet:    false,
		Required:     false,
		DefaultValue: nil,
	}

}

func (fdInfo *fieldInfo) writeSchema(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, `"`, fdInfo.fieldKey, `": {`)

	t++

	if !fdInfo.writeSchemaCollectionType(t, gen) {
		fdInfo.writeSchemaType(t, gen)
	}
	fdInfo.writeSchemaOptions(t, gen)
	fdInfo.writeSchemaElement(t, gen)

	t--

	t.P(gen, `},`)

}

func (fdInfo *fieldInfo) writeSchemaCollectionType(t tab, gen *protogen.GeneratedFile) bool {

	switch {

	case fdInfo.value.Desc.IsMap():
		t.P(gen, `Type: schema.TypeMap,`)
		return true

	case fdInfo.value.Desc.IsList():

		if fdInfo.schema.IsTypeSet {
			t.P(gen, `Type: schema.TypeSet,`)
			return true
		}

		t.P(gen, `Type: schema.TypeList,`)
		return true
	}

	return false

}

func (fdInfo *fieldInfo) writeSchemaType(t tab, gen *protogen.GeneratedFile) {

	msg := fdInfo.value.Desc.Message()

	switch fdInfo.value.Desc.Kind() {

	case protoreflect.BoolKind:
		t.P(gen, `Type: schema.TypeBool,`)

	case protoreflect.StringKind, protoreflect.BytesKind, protoreflect.EnumKind:
		t.P(gen, `Type: schema.TypeString,`)

	case protoreflect.MessageKind:

		switch msg.FullName() {

		case wellKnownDuration:
			t.P(gen, `Type: schema.TypeInt,`)

		default:
			t.P(gen, `Type: schema.TypeList,`)
			t.P(gen, `MaxItems: 1,`)

		}

	case protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind, protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Uint32Kind, protoreflect.Uint64Kind:
		t.P(gen, `Type: schema.TypeInt,`)

	case
		protoreflect.DoubleKind, protoreflect.FloatKind:
		t.P(gen, `Type: schema.TypeFloat,`)

	}

}

func (fdInfo *fieldInfo) writeSchemaElement(t tab, gen *protogen.GeneratedFile) {

	if isWellKnownMessage(fdInfo.value.Message) {
		return
	}

	kind := fdInfo.value.Desc.Kind()

	switch {

	case kind == protoreflect.MessageKind:

		t.P(gen, `Elem: &schema.Resource{`)

		t++

		mInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

		t.P(gen, `Schema: `, mInfo.prefixWithPackage(mInfo.schemaFunctionName), `(),`)

		t--

		t.P(gen, `},`)

	case fdInfo.value.Desc.IsMap(), fdInfo.value.Desc.IsList():
		t.P(gen, `Elem: &schema.Schema{`)

		t++

		fdInfo.writeSchemaType(t, gen)

		t--

		t.P(gen, `},`)

	}

}

func (fdInfo *fieldInfo) writeSchemaOptions(t tab, gen *protogen.GeneratedFile) {

	if fdInfo.schema.Required {
		t.P(gen, `Required: true,`)
	} else if fdInfo.schema.Computed {
		t.P(gen, `Computed: true,`)
	} else {
		t.P(gen, `Optional: true,`)
	}

	if fdInfo.value.Enum != nil {

		eInfo := newEnumInfo(fdInfo.value.Enum)

		eInfo.writeSchemaValidateFunc(t, gen)
	}

	if fdInfo.schema.DefaultValue != nil {

		switch val := fdInfo.schema.DefaultValue.Kind.(type) {

		case *structpb.Value_BoolValue:
			t.P(gen, `Default: `, fmt.Sprintf("%v", val.BoolValue), `,`)

		case *structpb.Value_NumberValue:
			t.P(gen, `Default: `, fmt.Sprintf("%v", val.NumberValue), `,`)

		case *structpb.Value_StringValue:

			fieldMsg := fdInfo.value.Desc.Message()

			if fieldMsg != nil && fieldMsg.FullName() == "google.protobuf.Duration" {

				duration, err := time.ParseDuration(val.StringValue)
				if err != nil {
					panic(err)
				}

				t.P(gen, `Default: time.Duration(`, duration.Nanoseconds(), `),`)

			} else {
				t.P(gen, "Default: `", val.StringValue, "`,")
			}

		}
	}

	if len(fdInfo.value.Comments.Leading) > 0 {
		t.P(gen, `Description: "`, commentToString(fdInfo.value.Comments.Leading), `",`)
	}

}

func (fdInfo *fieldInfo) writeUnmarshal(t tab, gen *protogen.GeneratedFile, sm selectorMaker) {

	selector := sm.makeSelector(fdInfo.fieldKey)

	collectionType := fdInfo.getFieldGoCollectionType()
	fieldType := fdInfo.getFieldGoType()

	if len(collectionType) > 0 {

		switch {

		case fdInfo.value.Desc.IsList():

			t.P(gen, `if `, fdInfo.valueVar, `, `, fdInfo.okVar, ` := `, selector, `.(`, collectionType, `); `, fdInfo.okVar, ` {`)

			t++

			if fdInfo.schema.IsTypeSet {
				t.P(gen, `list := `, fdInfo.valueVar, `.List()`)
			} else {
				t.P(gen, `list := `, fdInfo.valueVar)

			}

			t.P(gen, `r := []`, fieldType, `{}`)
			t.P(gen, `for _, val := range list {`)

			t++

			switch fdInfo.value.Desc.Kind() {

			case protoreflect.MessageKind:
				listMInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

				t.P(gen, `m, err :=  `, listMInfo.prefixWithPackage(listMInfo.unmarshalFunctionName), `(val.(map[string]interface{}))`)
				t.P(gen, `if err != nil {`)
				t++
				t.P(gen, `return nil, err`)
				t--
				t.P(gen, `}`)
				t.P(gen, `r = append(r, m)`)

			case protoreflect.BytesKind:
				t.P(gen, `r = append(r, []byte(val.(`, fieldType, `)))`)

				t.P(gen, `p["`, fdInfo.fieldKey, `"] = []byte(`, fdInfo.valueVar, `)`)

			default:
				t.P(gen, `r = append(r, val.(`, fieldType, `))`)

			}

			t--

			t.P(gen, `}`)
			t.P(gen, `p["`, fdInfo.fieldKey, `"] = r`)

			t--

			t.P(gen, `}`)

		case fdInfo.value.Desc.IsMap():

			t.P(gen, `if `, fdInfo.valueVar, `, `, fdInfo.okVar, ` := `, selector, `.(`, collectionType, `); `, fdInfo.okVar, `{`)

			t++

			mapValue := fdInfo.value.Desc.MapValue()

			t.P(gen, `m := map[string]interface{}`)
			t.P(gen, `for k, v := range `, fdInfo.valueVar, ` {`)

			t++

			switch mapValue.Kind() {

			case protoreflect.MessageKind:
				mapMInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

				t.P(gen, `m[k] = `, mapMInfo.prefixWithPackage(mapMInfo.unmarshalFunctionName), `(v.(map[string]interface{}))`)

			default:
				t.P(gen, `m[k] = v.(`, fieldType, `)`)

			}

			t--

			t.P(gen, `}`)
			t.P(gen, `p["`, fdInfo.fieldKey, `"] = m`)

			t--

			t.P(gen, `}`)

		case fdInfo.value.Desc.Kind() == protoreflect.MessageKind:

			t.P(gen, `if `, fdInfo.valueVar, `Collection, `, fdInfo.okVar, ` := `, selector, `.(`, collectionType, `); `, fdInfo.okVar, ` && len(`, fdInfo.valueVar, `Collection) > 0 {`)

			t++

			t.P(gen, `if `, fdInfo.valueVar, `, `, fdInfo.okVar, ` := `, fdInfo.valueVar, `Collection[0].(`, fieldType, `); `, fdInfo.okVar, ` {`)

			t++

			fieldMessageInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

			t.P(gen, `msg, err := `, fieldMessageInfo.prefixWithPackage(fieldMessageInfo.unmarshalFunctionName), `(`, fdInfo.valueVar, `)`)
			t.P(gen, `if err != nil {`)
			t++
			t.P(gen, `return nil, err`)
			t--
			t.P(gen, `}`)

			t.P(gen, `p["`, fdInfo.fieldKey, `"] = msg`)

			t--

			t.P(gen, `}`)

			t--

			t.P(gen, `}`)

		}

		return
	}

	switch fdInfo.value.Desc.Kind() {

	case protoreflect.BoolKind, protoreflect.StringKind, protoreflect.EnumKind, protoreflect.BytesKind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind, protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Uint32Kind,
		protoreflect.Uint64Kind, protoreflect.DoubleKind, protoreflect.FloatKind:

		t.P(gen, `if `, fdInfo.valueVar, `, `, fdInfo.okVar, ` := `, selector, `.(`, fieldType, `); `, fdInfo.okVar, ` {`)

		t++

		switch fdInfo.value.Desc.Kind() {

		case protoreflect.BytesKind:
			t.P(gen, `p["`, fdInfo.fieldKey, `"] = []byte(`, fdInfo.valueVar, `)`)

		default:
			t.P(gen, `p["`, fdInfo.fieldKey, `"] = `, fdInfo.valueVar)

		}

		t--

		t.P(gen, `}`)

	}

}

func (fdInfo *fieldInfo) getFieldGoCollectionType() string {

	msg := fdInfo.value.Desc.Message()

	switch {

	case fdInfo.value.Desc.IsList():

		if fdInfo.schema.IsTypeSet {
			return "*schema.Set"
		}

		return "[]interface{}"

	case fdInfo.value.Desc.IsMap():

		return "map[string]interface{}"

	case fdInfo.value.Desc.Kind() == protoreflect.MessageKind:

		switch msg.FullName() {

		case wellKnownDuration:
			return ""

		default:
			return "[]interface{}"

		}

	}

	return ""

}
func (fdInfo *fieldInfo) getFieldGoType() string {

	msg := fdInfo.value.Desc.Message()

	switch fdInfo.value.Desc.Kind() {

	case protoreflect.BoolKind:
		return "bool"

	case protoreflect.StringKind, protoreflect.BytesKind, protoreflect.EnumKind:
		return "string"

	case protoreflect.MessageKind:

		switch msg.FullName() {

		case wellKnownDuration:
			return "int"

		default:
			return "map[string]interface{}"

		}

	case protoreflect.Uint64Kind:
		return "uint64"

	case protoreflect.Uint32Kind:
		return "uint32"

	case protoreflect.Sfixed64Kind, protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Fixed64Kind:
		return "int64"

	case protoreflect.Sfixed32Kind, protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Fixed32Kind:
		return "int32"

	case
		protoreflect.FloatKind:
		return "float32"

	case
		protoreflect.DoubleKind:
		return "float64"

	}

	return "any"

}

func (fdInfo *fieldInfo) writeMarshal(t tab, gen *protogen.GeneratedFile, mi mapIndexMaker) {

	collectionType := fdInfo.getFieldGoCollectionType()
	fieldType := fdInfo.getFieldGoType()

	mapIndex := mi.makeMapIndex(fdInfo.fieldKey)

	if len(collectionType) > 0 {

		switch {

		case fdInfo.value.Desc.IsList():

			t.P(gen, `if l, ok := obj["`, fdInfo.fieldKey, `"].([]interface{}); ok {`)

			t++

			t.P(gen, mapIndex, ` = []interface{}{}`)
			t.P(gen, `for _, i := range l {`)

			t++

			switch {

			case fdInfo.value.Desc.Kind() == protoreflect.MessageKind:

				mInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

				t.P(gen, `d, err := `, mInfo.prefixWithPackage(mInfo.marshalFunctionName), `(i.(map[string]interface{}))`)
				t.P(gen, `if err != nil {`)
				t++
				t.P(gen, `return nil, err`)
				t--
				t.P(gen, `}`)

			default:
				t.P(gen, `d := i.(`, fieldType, `)`)

			}

			t.P(gen, `p["`, fdInfo.fieldKey, `"] = append(p["`, fdInfo.fieldKey, `"].([]interface{}), d)`)

			t--

			t.P(gen, `}`)

			t--

			t.P(gen, `}`)

		case fdInfo.value.Desc.IsMap():

			t.P(gen, `if m, ok := obj["`, fdInfo.fieldKey, `"].(map[string]interface{}); ok {`)

			t++

			t.P(gen, `p["`, fdInfo.fieldKey, `"] = map[string]interface{}{}`)
			t.P(gen, `for k, v := range m {`)

			t++

			switch {

			case fdInfo.value.Desc.Kind() == protoreflect.MessageKind:

				mInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

				t.P(gen, `d, err := `, mInfo.prefixWithPackage(mInfo.marshalFunctionName), `(i.(map[string]interface{}))`)
				t.P(gen, `if err != nil {`)
				t++
				t.P(gen, `return nil, err`)
				t--
				t.P(gen, `}`)

			default:
				t.P(gen, `d := i.(`, fieldType, `)`)

			}

			t.P(gen, mapIndex, `[k] = d`)

			t--

			t.P(gen, `}`)

			t--

			t.P(gen, `}`)

		case fdInfo.value.Desc.Kind() == protoreflect.MessageKind:

			mInfo := newMessageInfo(fdInfo.fInfo, fdInfo.value.Message)

			t.P(gen, `if m, ok := obj["`, fdInfo.fieldKey, `"].(map[string]interface{}); ok {`)

			t++

			t.P(gen, `d, err := `, mInfo.prefixWithPackage(mInfo.marshalFunctionName), `(m)`)
			t.P(gen, `if err != nil {`)
			t++
			t.P(gen, `return nil, err`)
			t--
			t.P(gen, `}`)
			t.P(gen, mapIndex, ` = []interface{}{d}`)

			t--

			t.P(gen, `}`)

		}

		return
	}
	switch fdInfo.value.Desc.Kind() {

	case protoreflect.BoolKind, protoreflect.StringKind, protoreflect.EnumKind, protoreflect.BytesKind,
		protoreflect.Sfixed32Kind, protoreflect.Sfixed64Kind, protoreflect.Fixed32Kind,
		protoreflect.Fixed64Kind, protoreflect.Int32Kind, protoreflect.Int64Kind,
		protoreflect.Sint32Kind, protoreflect.Sint64Kind, protoreflect.Uint32Kind,
		protoreflect.Uint64Kind, protoreflect.DoubleKind, protoreflect.FloatKind:

		switch fdInfo.value.Desc.Kind() {

		case protoreflect.BytesKind:
			t.P(gen, `if v, ok := obj["`, fdInfo.fieldKey, `"].(string); ok {`)
			t++
			t.P(gen, mapIndex, `, _ = []byte(v)`)
			t--
			t.P(gen, `}`)
		default:
			t.P(gen, mapIndex, `, _ = obj["`, fdInfo.fieldKey, `"].(`, fieldType, `)`)

		}

	}

}
