package main

import (
	"fmt"
	"path"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
)

type importNeeds struct {
	needTime       bool
	needSchema     bool
	needValidation bool
	needEncoding   bool

	customImports   map[string]string
	customImportMap map[string]string

	usedCustomImports map[string]string
}

func newImportNeeds() *importNeeds {
	return &importNeeds{
		needTime:          false,
		needSchema:        false,
		needValidation:    false,
		needEncoding:      false,
		customImports:     make(map[string]string),
		customImportMap:   make(map[string]string),
		usedCustomImports: make(map[string]string),
	}
}

func (in *importNeeds) getPackageForMessage(msg *protogen.Message) string {

	fullName := string(msg.Desc.FullName())

	if pathSpec, ok := in.customImportMap[fullName]; ok {

		parts := strings.Split(pathSpec, ";")

		switch len(parts) {

		case 0:
			panic("invalid import map")

		case 1:
			parts = append(parts, path.Base(parts[0]))

		}

		in.usedCustomImports[fmt.Sprintf(`"%s"`, parts[0])] = parts[1]

		return fmt.Sprintf("%s.", parts[1])

	}

	return ""

}

func (in *importNeeds) getPackage(i protogen.GoIdent) string {

	ip := string(i.GoImportPath)

	if p, ok := in.customImports[ip]; ok {
		in.usedCustomImports[ip] = p
		return fmt.Sprintf("%s.", p)
	}

	return ""

}

func (in *importNeeds) discoverFiles(files []*protogen.File) {

	for _, f := range files {
		in.customImports[f.GoImportPath.String()] = string(f.GoPackageName)
	}

}

func (in *importNeeds) discoverMessage(msg *protogen.Message) {

	in.needSchema = true
	in.needEncoding = true

	if len(msg.Enums) > 0 {
		in.needValidation = true
	}

	in.getPackageForMessage(msg)

	for _, field := range msg.Fields {

		fdInfo := newFieldInfo(nil, field)

		if field.Message != nil {

			in.getPackageForMessage(field.Message)

			switch field.Message.Desc.FullName() {

			case wellKnownDuration:
				if fdInfo.schema.DefaultValue != nil && fdInfo.schema.DefaultValue.AsInterface() != nil {
					in.needTime = true
				}

			}

		}

		if field.Enum != nil || field.Desc.Enum() != nil {
			in.needValidation = true
		}

	}

}

func (in *importNeeds) writeFile(t tab, gen *protogen.GeneratedFile) {

	t.P(gen, "import (")

	t++

	if in.needTime {
		t.P(gen, `"time"`)
	}

	if in.needSchema {
		t.P(gen, `"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"`)
	}

	if in.needValidation {
		t.P(gen, `"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"`)
	}

	if in.needEncoding {
		t.P(gen, `"google.golang.org/protobuf/encoding/protojson"`)
		t.P(gen, `"google.golang.org/protobuf/proto"`)
		t.P(gen, `"encoding/json"`)
		t.P(gen, `"reflect"`)
	}

	for filePath, packageName := range in.usedCustomImports {
		t.P(gen, fmt.Sprintf(`%s %s`, packageName, filePath))
	}

	t--

	t.P(gen, ")")

}
