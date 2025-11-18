package http

import (
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/protobuf/types/known/structpb"
)

type plainTextMarshaler struct {
	runtime.JSONPb // 继承默认 JSON 序列化行为
}

func (m *plainTextMarshaler) Marshal(v interface{}) ([]byte, error) {
	// 仅当返回类型是 structpb.Value 且为 string 时，按纯文本输出
	if val, ok := v.(*structpb.Value); ok {
		if s, ok := val.Kind.(*structpb.Value_StringValue); ok {
			return []byte(s.StringValue), nil
		}
	}

	// 其他类型正常走 JSON
	return m.JSONPb.Marshal(v)
}

func (m *plainTextMarshaler) ContentType(_ interface{}) string {
	return "text/plain; charset=utf-8"
}
