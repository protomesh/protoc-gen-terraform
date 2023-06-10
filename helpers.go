package main

import (
	"regexp"
	"strings"

	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

const (
	wellKnownDuration = "google.protobuf.Duration"
)

func isWellKnownMessage(msg *protogen.Message) bool {

	if msg == nil {
		return false
	}

	switch msg.Desc.FullName() {

	case wellKnownDuration:
		return true

	}

	return false

}

func getDescriptorFullName(desc protoreflect.Descriptor, delimiter string) string {

	parts := []string{}

	currentDesc := desc.Parent()

	parents := []string{}

	for {
		parentDesc, ok := currentDesc.(protoreflect.MessageDescriptor)
		if !ok || parentDesc == nil {
			break
		}
		parents = append(parents, string(parentDesc.Name()))
		currentDesc = parentDesc.Parent()
	}

	for i := len(parents) - 1; i >= 0; i-- {
		parts = append(parts, parents[i])
	}

	return strings.Join(append(parts,
		strcase.ToCamel(string(desc.Name())),
	), delimiter)

}

type tab int

func (t tab) String() string {
	return strings.Repeat("\t", int(t))
}

func (t tab) P(gen *protogen.GeneratedFile, v ...interface{}) {
	gen.P(append([]interface{}{t.String()}, v...)...)
}

func commentToString(c protogen.Comments) string {

	comments := string(c)
	comments = regexp.MustCompile(`[\r\n]`).ReplaceAllString(comments, " ")
	comments = regexp.MustCompile(`["]`).ReplaceAllString(comments, "'")
	comments = strings.Trim(comments, " ")

	return comments
}

func getDescriptorParentFile(desc protoreflect.Descriptor) protoreflect.FileDescriptor {

	currentDesc := desc

	for {
		parentDesc, ok := currentDesc.(protoreflect.FileDescriptor)
		if ok {
			currentDesc = parentDesc
			break
		}
		currentDesc = parentDesc.Parent()
		if currentDesc == nil {
			return nil
		}
	}

	return currentDesc.(protoreflect.FileDescriptor)

}
