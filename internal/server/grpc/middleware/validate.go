package middleware

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/x-thooh/delay/api/proto/validate" // 自定义 PGV 扩展
	"github.com/x-thooh/delay/pkg/log"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func UnaryServerValidatorInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {

		if msg, ok := req.(proto.Message); ok {
			if err := ValidateWithCustomError(msg); err != nil {
				logger.Error(ctx, "验证失败", "error", err.Error())
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		}

		return handler(ctx, req)
	}
}

func StreamServerValidatorInterceptor(logger log.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// 流式消息的验证通常在服务实现中处理
		logger.Info(ss.Context(), "收到流式请求，验证应在服务实现中处理", "method", info.FullMethod)
		return handler(srv, ss)
	}
}

// 获取字段级自定义错误
func getFieldCustomError(msg proto.Message, field string, defaultErr string) string {
	desc := msg.ProtoReflect().Descriptor()
	protoName := camelToSnake(field)
	fd := desc.Fields().ByName(protoreflect.Name(protoName))
	if fd != nil {
		if ext := proto.GetExtension(fd.Options(), validate.E_CustomError); ext != nil {
			if ce, ok := ext.(string); ok && ce != "" {
				return ce
			}
		}
	}
	return defaultErr
}

// 驼峰转下划线，例如 DelayTime -> delay_time
func camelToSnake(in string) string {
	out := ""
	for i, r := range in {
		if i > 0 && r >= 'A' && r <= 'Z' {
			out += "_" + string(r+'a'-'A')
		} else {
			out += strings.ToLower(string(r))
		}
	}
	return out
}

// FieldError 可抽象 PGV 单字段错误
type FieldError interface {
	Field() string
	Reason() string
}

// MultiError 可抽象 PGV 多字段错误
type MultiError interface {
	AllErrors() []error
}

// parseValidateError 通过断言将 PGV Validate 错误解析为字段 -> 错误映射
func parseValidateError(msg proto.Message, err error) map[string]string {
	fieldErrs := make(map[string]string)
	if err == nil {
		return fieldErrs
	}

	// 先判断是否 MultiError
	if me, ok := err.(MultiError); ok {
		for _, sub := range me.AllErrors() {
			subErrs := parseValidateError(msg, sub)
			for k, v := range subErrs {
				fieldErrs[k] = v
			}
		}
		return fieldErrs
	}

	// 判断是否 FieldError
	if fe, ok := err.(FieldError); ok {
		field := fe.Field()
		reason := fe.Reason()
		// 尝试获取 custom_error
		if ce := getFieldCustomError(msg, field, reason); ce != "" {
			fieldErrs[field] = ce
		} else {
			fieldErrs[field] = reason
		}
		return fieldErrs
	}

	// 普通错误，无法映射字段
	fieldErrs["_"] = err.Error()
	return fieldErrs
}

// ValidateWithCustomError 调用 PGV Validate 并返回中文错误
func ValidateWithCustomError(msg proto.Message) error {
	validateAll, ok := msg.(interface{ ValidateAll() error })
	if !ok {
		return nil
	}
	if err := validateAll.ValidateAll(); err != nil {
		fieldErrs := parseValidateError(msg, err)
		// 拼接成一个错误字符串
		msgs := make([]string, 0, len(fieldErrs))
		for f, e := range fieldErrs {
			if f == "_" {
				msgs = append(msgs, e)
			} else {
				msgs = append(msgs, fmt.Sprintf("%s: %s", f, e))
			}
		}
		return errors.New(strings.Join(msgs, ";"))
	}
	return nil
}
