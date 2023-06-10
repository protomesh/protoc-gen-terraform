package main

import (
	"strings"

	terraformpb "github.com/protomesh/protoc-gen-terraform/proto/terraform"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type enumInfo struct {
	value  *protogen.Enum
	schema *terraformpb.EnumSchema

	typeName string
	prefix   string
}

func newEnumInfo(value *protogen.Enum) *enumInfo {

	fullName := getDescriptorFullName(value.Desc, "_")

	return &enumInfo{
		value:  value,
		schema: getEnumSchema(value.Desc),

		typeName: fullName,
		prefix:   fullName,
	}
}

func getEnumSchema(desc protoreflect.EnumDescriptor) *terraformpb.EnumSchema {

	opts, ok := desc.Options().(*descriptorpb.EnumOptions)
	if !ok {
		panic("Invalid message options")
	}

	if opts != nil && proto.HasExtension(opts, terraformpb.E_EnumSchema) {

		return proto.GetExtension(opts, terraformpb.E_EnumSchema).(*terraformpb.EnumSchema)

	}

	return &terraformpb.EnumSchema{}

}

func (eInfo *enumInfo) writeSchemaValidateFunc(t tab, gen *protogen.GeneratedFile) {

	possibleValues := []string{}

	for _, value := range eInfo.value.Values {

		possibleValues = append(possibleValues, string(value.Desc.Name()))

	}

	t.P(gen, `ValidateFunc: validation.StringInSlice([]string{"`, strings.Join(possibleValues, `","`), `"}, false),`)

}
