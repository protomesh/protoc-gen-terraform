package protobuf

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func NewValueSchema() map[string]*schema.Schema {
	return map[string]*schema.Schema{
		"value": {
			Type:        schema.TypeString,
			Optional:    true,
			Description: "Any valid JSON value, as number, boolean, null, quoted string, array or struct. See https://github.com/protocolbuffers/protobuf/blob/main/src/google/protobuf/struct.proto for more information.",
		},
	}
}

func UnmarshalValue(obj map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := obj["value"].(string); ok {
		return obj, nil
	}
	return map[string]interface{}{"value": "null"}, nil
}

func UnmarshalValueProto(obj map[string]interface{}, m proto.Message) error {
	d, err := UnmarshalValue(obj)
	if err != nil {
		return err
	}
	if err := protojson.Unmarshal([]byte(d["value"].(string)), m); err != nil {
		return err
	}
	return nil
}

func MarshalValue(obj map[string]interface{}) (map[string]interface{}, error) {
	if _, ok := obj["value"].(string); ok {
		return obj, nil
	}
	return map[string]interface{}{"value": "null"}, nil
}

func MarshalValueProto(m proto.Message) (map[string]interface{}, error) {
	obj := map[string]interface{}{}
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(m)
	if err != nil {
		return nil, err
	}
	obj["value"] = string(b)
	return MarshalValue(obj)
}
