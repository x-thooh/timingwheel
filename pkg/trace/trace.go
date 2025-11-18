package trace

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// 定义 traceID key（slog 的 Attr key）
const (
	traceIDCtxKey    = "trace_id"
	traceIDHeaderKey = "X-Trace-ID"
)

func GetCtxKey() string {
	return traceIDCtxKey
}

func GetHeaderKey() string {
	return traceIDHeaderKey
}

func GenerateTraceID() string {
	return strings.Replace(uuid.New().String(), "-", "", -1)
}

func Append(ctx context.Context, traceId string) context.Context {
	if len(traceId) == 0 {
		return ctx
	}
	current := Get(ctx)
	if len(current) == 0 {
		return Set(ctx, traceId)
	}
	return context.WithValue(ctx, GetCtxKey(), []string{current, traceId})
}

func Set(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, GetCtxKey(), traceId)
}

func Get(ctx context.Context) string {
	tid := ctx.Value(GetCtxKey())
	switch tid.(type) {
	case string:
		return tid.(string)
	case []string:
		return strings.Join(tid.([]string), ",")
	}
	return ""
}
