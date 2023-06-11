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

type oneOfInfo struct {
	fInfo *fileInfo

	value  *protogen.Oneof
	schema *terraformpb.OneofSchema

	oneOfKey string

	okVar    string
	valueVar string

	marshalFunctionName   string
	unmarshalFunctionName string
}

func newOneOfInfo(fInfo *fileInfo, value *protogen.Oneof) *oneOfInfo {

	name := string(value.Desc.Name())
	varName := strcase.ToCamel(name)
	fullName := getDescriptorFullName(value.Desc, "")

	return &oneOfInfo{
		fInfo: fInfo,

		value:  value,
		schema: getOneofSchema(value.Desc),

		oneOfKey: name,

		okVar:    fmt.Sprintf("ok%s", varName),
		valueVar: fmt.Sprintf("value%s", varName),

		marshalFunctionName:   fmt.Sprintf("Marshal%s", fullName),
		unmarshalFunctionName: fmt.Sprintf("Unmarshal%s", fullName),
	}
}

func getOneofSchema(desc protoreflect.OneofDescriptor) *terraformpb.OneofSchema {

	opts, ok := desc.Options().(*descriptorpb.OneofOptions)
	if !ok {
		panic("Invalid message options")
	}

	if opts != nil && proto.HasExtension(opts, terraformpb.E_OneofSchema) {

		return proto.GetExtension(opts, terraformpb.E_OneofSchema).(*terraformpb.OneofSchema)

	}

	return &terraformpb.OneofSchema{}

}

func (oInfo *oneOfInfo) writeSchema(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, `"`, oInfo.oneOfKey, `": {`)

	t++

	t.P(gen, `Type: schema.TypeList,`)
	t.P(gen, `MaxItems: 1,`)
	t.P(gen, `Optional: true,`)
	t.P(gen, `Elem: &schema.Resource{`)

	t++

	t.P(gen, `Schema: map[string]*schema.Schema{`)

	t++

	for _, field := range oInfo.value.Fields {

		fdInfo := newFieldInfo(oInfo.fInfo, field)

		fdInfo.writeSchema(t, gen)

	}

	t--

	t.P(gen, `},`)

	t--

	t.P(gen, `},`)

	t--

	t.P(gen, `},`)

}

func (oInfo *oneOfInfo) makeSelector(fieldKey string) string {
	return "oneOfVal"
}

func (oInfo *oneOfInfo) makeMapIndex(fieldKey string) string {
	return fmt.Sprintf(`p["%s"].([]interface{})[0].(map[string]interface{})["%s"]`, oInfo.oneOfKey, fieldKey)
}

func (oInfo *oneOfInfo) writeUnmarshal(t tab, gen *protogen.GeneratedFile, sm selectorMaker) {

	selector := sm.makeSelector(oInfo.oneOfKey)

	t.P(gen, `if `, oInfo.valueVar, `, `, oInfo.okVar, ` := `, selector, `.([]interface{}); `, oInfo.okVar, ` && len(`, oInfo.valueVar, `) > 0 {`)

	t++

	t.P(gen, `o := `, oInfo.valueVar, `[0].(map[string]interface{})`)

	cond := `if`

	for _, f := range oInfo.value.Fields {

		oneOfFInfo := newFieldInfo(oInfo.fInfo, f)

		t.P(gen, cond, ` oneOfVal, ok := o["`, oneOfFInfo.fieldKey, `"]; ok {`)
		t++
		oneOfFInfo.writeUnmarshal(t, gen, oInfo)
		t--

		if len(oInfo.value.Fields) > 1 {
			cond = `} else if`
		}

	}

	t.P(gen, `}`)

	t--

	t.P(gen, `}`)

}

func (oInfo *oneOfInfo) writeMarshal(t tab, gen *protogen.GeneratedFile, mi mapIndexMaker) {

	selector := mi.makeMapIndex(oInfo.oneOfKey)

	t.P(gen, selector, ` = []interface{}{}`)

	cond := `if`

	for _, f := range oInfo.value.Fields {

		oneOfFInfo := newFieldInfo(oInfo.fInfo, f)

		t.P(gen, cond, ` _, ok := obj["`, oneOfFInfo.fieldKey, `"]; ok {`)
		t++
		t.P(gen, selector, ` = append(`, selector, `.([]interface{}), map[string]interface{}{})`)
		oneOfFInfo.writeMarshal(t, gen, oInfo)
		t--

		if len(oInfo.value.Fields) > 1 {
			cond = `} else if`
		}

	}

	t.P(gen, `}`)

}
