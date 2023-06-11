package main

import (
	"fmt"

	"github.com/iancoleman/strcase"
	terraformpb "github.com/protomesh/protoc-gen-terraform/proto/terraform"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type messageInfo struct {
	fInfo *fileInfo

	value  *protogen.Message
	schema *terraformpb.MessageSchema

	marshalFunctionName   string
	unmarshalFunctionName string
	schemaFunctionName    string

	okVar    string
	valueVar string

	sourceVar string
	source    string
	selector  string
}

func newMessageInfo(fInfo *fileInfo, value *protogen.Message) *messageInfo {

	fullName := getDescriptorFullName(value.Desc, "")
	varName := strcase.ToLowerCamel(fullName)

	// panic(fmt.Sprintf("%s\n%s", value.Location.Path.String(), value.GoIdent.GoImportPath, value.GoIdent.GoName))

	mInfo := &messageInfo{
		fInfo: fInfo,

		value:  value,
		schema: getMessageSchema(value.Desc),

		marshalFunctionName:   fmt.Sprintf("Marshal%s", fullName),
		unmarshalFunctionName: fmt.Sprintf("Unmarshal%s", fullName),
		schemaFunctionName:    fmt.Sprintf("New%sSchema", fullName),

		okVar:    fmt.Sprintf("ok%s", varName),
		valueVar: fmt.Sprintf("value%s", varName),

		sourceVar: "obj",
		source:    "obj map[string]interface{}",
		selector:  "obj[\"%s\"]",
	}

	return mInfo
}

func getMessageSchema(desc protoreflect.MessageDescriptor) *terraformpb.MessageSchema {

	opts, ok := desc.Options().(*descriptorpb.MessageOptions)
	if !ok {
		panic("Invalid message options")
	}

	if opts != nil && proto.HasExtension(opts, terraformpb.E_MessageSchema) {

		// https://stackoverflow.com/questions/28815214/how-to-set-get-protobufs-extension-field-in-go
		return proto.GetExtension(opts, terraformpb.E_MessageSchema).(*terraformpb.MessageSchema)

	}

	return &terraformpb.MessageSchema{
		Generate:   false,
		IsResource: false,
	}

}

func (mInfo *messageInfo) prefixWithPackage(suffix string) string {

	if mInfo.fInfo.file.GoImportPath != mInfo.value.GoIdent.GoImportPath {

		i := mInfo.fInfo.importNeeds.getPackageForMessage(mInfo.value)

		if len(i) == 0 {
			i = mInfo.fInfo.importNeeds.getPackage(mInfo.value.GoIdent)
		}

		suffix = fmt.Sprintf("%s%s", i, suffix)
	}

	return suffix
}

func (mInfo *messageInfo) makeSelector(fieldKey string) string {
	return fmt.Sprintf(mInfo.selector, fieldKey)
}

func (mInfo *messageInfo) makeMapIndex(fieldKey string) string {
	return fmt.Sprintf(`p["%s"]`, fieldKey)
}

func (mInfo *messageInfo) writeSchemaFunction(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, `func `, mInfo.schemaFunctionName, `() map[string]*schema.Schema {`)

	t++

	t.P(gen, `return map[string]*schema.Schema{`)

	t++

	for _, field := range mInfo.value.Fields {

		if field.Oneof == nil {

			fdInfo := newFieldInfo(mInfo.fInfo, field)

			fdInfo.writeSchema(t, gen)

		}

	}

	for _, oneOf := range mInfo.value.Oneofs {

		oInfo := newOneOfInfo(mInfo.fInfo, oneOf)

		oInfo.writeSchema(t, gen)

	}

	t--

	t.P(gen, `}`)

	t--

	t.P(gen, `}`)

}

func (mInfo *messageInfo) writeMarshaler(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, `func `, mInfo.marshalFunctionName, `(obj map[string]interface{}) (map[string]interface{}, error) {`)

	t++

	t.P(gen, `p := map[string]interface{}{}`)

	for _, field := range mInfo.value.Fields {

		if field.Oneof == nil {

			fdInfo := newFieldInfo(mInfo.fInfo, field)

			fdInfo.writeMarshal(t, gen, mInfo)

		}

	}

	for _, oneOf := range mInfo.value.Oneofs {

		oInfo := newOneOfInfo(mInfo.fInfo, oneOf)

		oInfo.writeMarshal(t, gen, mInfo)

	}

	t.P(gen, `return p, nil`)

	t--

	t.P(gen, `}`)

	gen.P()
	// Proto

	t.P(gen, `func `, mInfo.marshalFunctionName, `Proto(m proto.Message) (map[string]interface{}, error) {`)

	t++

	t.P(gen, `obj := map[string]interface{}{}`)

	t.P(gen, `b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)`)
	t.P(gen, `if err != nil {`)
	t++
	t.P(gen, `return nil, err`)
	t--
	t.P(gen, `}`)

	t.P(gen, `err = json.Unmarshal(b, &obj)`)
	t.P(gen, `if err != nil {`)
	t++
	t.P(gen, `return nil, err`)
	t--
	t.P(gen, `}`)

	t.P(gen, `return `, mInfo.marshalFunctionName, `(obj)`)

	t--

	t.P(gen, `}`)

	gen.P()

	if mInfo.schema.IsResource {

		t.P(gen, `func `, mInfo.marshalFunctionName, `ResourceData(m proto.Message, rd *schema.ResourceData) {`)

		t++

		t.P(gen, `pMap := `, mInfo.marshalFunctionName, `Proto(m)`)
		t.P(gen, `for k, v := range pMap {`)
		t++
		t.P(gen, `rd.Set(k, v)`)
		t--
		t.P(gen, `}`)

		t--

		t.P(gen, `}`)

		gen.P()

	}

}

func (mInfo *messageInfo) writeUnmarshaler(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, `func `, mInfo.unmarshalFunctionName, `(`, mInfo.source, `) (map[string]interface{}, error) {`)

	t++

	t.P(gen, `p := map[string]interface{}{}`)

	for _, field := range mInfo.value.Fields {

		if field.Oneof == nil {

			fdInfo := newFieldInfo(mInfo.fInfo, field)

			fdInfo.writeUnmarshal(t, gen, mInfo)

		}

	}

	for _, oneOf := range mInfo.value.Oneofs {

		oInfo := newOneOfInfo(mInfo.fInfo, oneOf)

		oInfo.writeUnmarshal(t, gen, mInfo)

	}

	t--

	t.P(gen, `return p, nil`)
	t.P(gen, `}`)

	gen.P()

	// Proto unmarshal

	t.P(gen, `func `, mInfo.unmarshalFunctionName, `Proto (`, mInfo.source, `, m proto.Message) error {`)

	t++

	t.P(gen, `d, err := `, mInfo.prefixWithPackage(mInfo.unmarshalFunctionName), `(`, mInfo.sourceVar, `)`)
	t.P(gen, `if err != nil {`)
	t++
	t.P(gen, `return err`)
	t--
	t.P(gen, `}`)

	t.P(gen, `b, err := json.Marshal(d)`)
	t.P(gen, `if err != nil {`)
	t++
	t.P(gen, `return err`)
	t--
	t.P(gen, `}`)

	t.P(gen, `if err := protojson.Unmarshal(b, m); err != nil {`)
	t++
	t.P(gen, `return err`)
	t--
	t.P(gen, `}`)

	t.P(gen, `return nil`)

	t--

	t.P(gen, `}`)

	gen.P()

	if mInfo.schema.IsResource {

		if mInfo.schema.IsResource {
			mInfo.sourceVar = "rd"
			mInfo.source = "rd *schema.ResourceData"
			mInfo.selector = "rd.Get(\"%s\")"
		}

		t.P(gen, `func `, mInfo.unmarshalFunctionName, `ResourceData(`, mInfo.source, `) (map[string]interface{}, error) {`)

		t++

		t.P(gen, `p := map[string]interface{}{}`)

		for _, field := range mInfo.value.Fields {

			if field.Oneof == nil {

				fdInfo := newFieldInfo(mInfo.fInfo, field)

				fdInfo.writeUnmarshal(t, gen, mInfo)

			}

		}

		for _, oneOf := range mInfo.value.Oneofs {

			oInfo := newOneOfInfo(mInfo.fInfo, oneOf)

			oInfo.writeUnmarshal(t, gen, mInfo)

		}

		t--

		t.P(gen, `return p, nil`)
		t.P(gen, `}`)

		gen.P()

	}

}
