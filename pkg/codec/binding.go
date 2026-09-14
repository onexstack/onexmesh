// Copyright 2026 Lingfei Kong <colin404@foxmail.com>. All rights reserved.
// Use of this source code is governed by a MIT style
// license that can be found in the LICENSE file.

package codec

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// BindPath populates msg from the path parameters declared by template. A
// template placeholder "{name}" maps to the request message field "name" (via
// gin's c.Param("name")). Path values are strings; scalar fields are coerced
// using the same rules as BindQuery. Unknown placeholders are ignored so a
// single template may carry multiple parameters.
func BindPath(c *gin.Context, msg proto.Message, template string) error {
	m := msg.ProtoReflect()
	for _, field := range pathFields(template) {
		if err := populateField(m, field, c.Param(field)); err != nil {
			return fmt.Errorf("codec: bind path param %q: %w", field, err)
		}
	}
	return nil
}

// BindQuery populates msg from the URL query parameters. Each query key names a
// request message field (snake_case or lowerCamelCase); values are coerced to
// the field's scalar type, enum name, repeated scalar, or a nested message field
// keyed by dot ("a.b=1"). Unknown query keys are ignored.
func BindQuery(c *gin.Context, msg proto.Message) error {
	m := msg.ProtoReflect()
	for key, values := range c.Request.URL.Query() {
		if err := populateField(m, key, values[len(values)-1]); err != nil {
			return fmt.Errorf("codec: bind query param %q: %w", key, err)
		}
	}
	return nil
}

// pathFields extracts the "{field}" placeholders from a path template, in
// order, ignoring the "=glob" suffix of the "{name=messages/*}" form.
func pathFields(template string) []string {
	var fields []string
	for i := 0; i < len(template); i++ {
		if template[i] != '{' {
			continue
		}
		end := strings.IndexByte(template[i:], '}')
		if end < 0 {
			break
		}
		field := template[i+1 : i+end]
		if eq := strings.IndexByte(field, '='); eq >= 0 {
			field = field[:eq]
		}
		if field != "" {
			fields = append(fields, field)
		}
		i += end
	}
	return fields
}

// populateField sets the field named by name (a dotted path into nested
// messages) on m to the string value. Scalars are coerced by strconv; enums by
// name; repeated scalar fields by comma-splitting value; nested messages recurse
// on the last dotted segment.
func populateField(m protoreflect.Message, name, value string) error {
	parts := strings.Split(name, ".")
	field := m.Descriptor().Fields().ByName(protoreflect.Name(parts[0]))
	if field == nil {
		// Try lowerCamelCase (JSON) form before giving up.
		if field = m.Descriptor().Fields().ByJSONName(parts[0]); field == nil {
			return nil // unknown field: ignore, matching grpc-gateway
		}
	}

	if len(parts) > 1 {
		if field.Kind() != protoreflect.MessageKind {
			return fmt.Errorf("field %s is not a message", parts[0])
		}
		sub := m.Mutable(field).Message()
		return populateField(sub, strings.Join(parts[1:], "."), value)
	}

	if field.IsList() {
		return populateList(m, field, value)
	}
	return setScalar(m, field, value)
}

// populateList sets a repeated field from a comma-separated value.
func populateList(m protoreflect.Message, field protoreflect.FieldDescriptor, value string) error {
	list := m.Mutable(field).List()
	if value == "" {
		return nil
	}
	for _, item := range strings.Split(value, ",") {
		item = strings.TrimSpace(item)
		v, err := parseScalar(field, item)
		if err != nil {
			return err
		}
		list.Append(v)
	}
	return nil
}

// setScalar sets a singular scalar/enum/message field from a string.
func setScalar(m protoreflect.Message, field protoreflect.FieldDescriptor, value string) error {
	v, err := parseScalar(field, value)
	if err != nil {
		return err
	}
	m.Set(field, v)
	return nil
}

// parseScalar parses a string into the value type expected by field.
func parseScalar(field protoreflect.FieldDescriptor, s string) (protoreflect.Value, error) {
	switch field.Kind() {
	case protoreflect.StringKind:
		return protoreflect.ValueOfString(s), nil
	case protoreflect.BytesKind:
		return protoreflect.ValueOfBytes([]byte(s)), nil
	case protoreflect.BoolKind:
		b, err := strconv.ParseBool(s)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse bool: %w", err)
		}
		return protoreflect.ValueOfBool(b), nil
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind:
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse int32: %w", err)
		}
		return protoreflect.ValueOfInt32(int32(n)), nil
	case protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse int64: %w", err)
		}
		return protoreflect.ValueOfInt64(n), nil
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind:
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse uint32: %w", err)
		}
		return protoreflect.ValueOfUint32(uint32(n)), nil
	case protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		n, err := strconv.ParseUint(s, 10, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse uint64: %w", err)
		}
		return protoreflect.ValueOfUint64(n), nil
	case protoreflect.FloatKind:
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse float: %w", err)
		}
		return protoreflect.ValueOfFloat32(float32(f)), nil
	case protoreflect.DoubleKind:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return protoreflect.Value{}, fmt.Errorf("parse double: %w", err)
		}
		return protoreflect.ValueOfFloat64(f), nil
	case protoreflect.EnumKind:
		val := field.Enum().Values().ByName(protoreflect.Name(s))
		if val == nil {
			return protoreflect.Value{}, fmt.Errorf("unknown enum value %q", s)
		}
		return protoreflect.ValueOfEnum(val.Number()), nil
	default:
		return protoreflect.Value{}, fmt.Errorf("unsupported field kind %s", field.Kind())
	}
}
