package main

import (
	terraformpb "github.com/protomesh/protoc-gen-terraform/proto/terraform"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

type fileInfo struct {
	importNeeds *importNeeds

	file   *protogen.File
	schema *terraformpb.FileSchema

	messages map[string]*protogen.Message
}

func newFileInfo(file *protogen.File) *fileInfo {

	fInfo := &fileInfo{
		importNeeds: newImportNeeds(),
		file:        file,
		schema:      getFileSchema(file.Desc),
		messages:    make(map[string]*protogen.Message),
	}

	fInfo.importNeeds.customImportMap = fInfo.schema.ImportMap

	return fInfo
}

func (fInfo *fileInfo) discoverMessage(msg *protogen.Message) {

	fInfo.messages[string(msg.Desc.FullName())] = msg

	fInfo.importNeeds.discoverMessage(msg)

	for _, nested := range msg.Messages {
		fInfo.discoverMessage(nested)
	}

}

func (fInfo *fileInfo) discoverFile() {

	if len(fInfo.file.Enums) > 0 {
		fInfo.importNeeds.needValidation = true
	}

	for _, msg := range fInfo.file.Messages {

		msgOpts := getMessageSchema(msg.Desc)

		if msgOpts.Generate {

			fInfo.discoverMessage(msg)

		}
	}

}

func (fInfo *fileInfo) writeFunctions(t tab, gen *protogen.GeneratedFile) {

	for _, msg := range fInfo.messages {

		mInfo := newMessageInfo(fInfo, msg)

		mInfo.writeSchemaFunction(t, gen)
		gen.P()

		mInfo.writeUnmarshaler(t, gen)
		gen.P()

	}

}

func getFileSchema(desc protoreflect.FileDescriptor) *terraformpb.FileSchema {

	opts, ok := desc.Options().(*descriptorpb.FileOptions)
	if !ok {
		panic("Invalid message options")
	}

	if opts != nil && proto.HasExtension(opts, terraformpb.E_FileSchema) {

		// https://stackoverflow.com/questions/28815214/how-to-set-get-protobufs-extension-field-in-go
		return proto.GetExtension(opts, terraformpb.E_FileSchema).(*terraformpb.FileSchema)

	}

	return &terraformpb.FileSchema{
		ImportMap: make(map[string]string),
	}

}
