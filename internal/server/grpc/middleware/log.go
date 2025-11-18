package middleware

import (
	"context"
	"time"

	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
)

// UnaryServerLogInterceptor 创建一个gRPC一元服务器拦截器
func UnaryServerLogInterceptor(logger log.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		// 开始时间
		startTime := time.Now()

		// 从元数据中提取traceID，如果没有则生成新的
		md, ok := metadata.FromIncomingContext(ctx)
		var traceID string
		if ok {
			traceIDs := md.Get(trace.GetHeaderKey())
			if len(traceIDs) > 0 {
				traceID = traceIDs[0]
			}
		}

		if traceID == "" {
			traceID = trace.GenerateTraceID()
		}

		// 创建带有traceID的新上下文
		ctx = trace.Set(ctx, traceID)
		// 获取客户端信息
		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		// 记录请求信息
		logger.Info(ctx, "GRPC request received",
			"method", info.FullMethod,
			"client_ip", clientIP,
		)

		// 调用实际的处理函数
		resp, err := handler(ctx, req)

		// 计算处理时间
		duration := time.Since(startTime)

		// 记录响应信息
		if err != nil {
			logger.Error(ctx, "GRPC request failed",
				"method", info.FullMethod,
				"client_ip", clientIP,
				"duration_ms", duration.Milliseconds(),
				"error", err.Error(),
			)
		} else {
			logger.Info(ctx, "GRPC request completed",
				"method", info.FullMethod,
				"client_ip", clientIP,
				"duration_ms", duration.Milliseconds(),
			)
		}

		return resp, err
	}
}

// StreamServerLogInterceptor 创建一个gRPC流服务器拦截器
func StreamServerLogInterceptor(logger log.Logger) grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		// 开始时间
		startTime := time.Now()

		// 从元数据中提取traceID，如果没有则生成新的
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		var traceID string
		if ok {
			traceIDs := md.Get(trace.GetHeaderKey())
			if len(traceIDs) > 0 {
				traceID = traceIDs[0]
			}
		}

		if traceID == "" {
			traceID = trace.GenerateTraceID()
		}

		// 创建带有traceID的新上下文
		ctx = trace.Set(ctx, traceID)

		// 获取客户端信息
		clientIP := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			clientIP = p.Addr.String()
		}

		// 记录请求信息
		logger.Info(ctx, "GRPC stream request received",
			"method", info.FullMethod,
			"client_ip", clientIP,
		)

		// 创建一个包装的ServerStream以传递新的上下文
		wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: ctx}

		// 调用实际的处理函数
		err := handler(srv, wrappedStream)

		// 计算处理时间
		duration := time.Since(startTime)

		// 记录响应信息
		if err != nil {
			logger.Error(ctx, "GRPC stream request failed",
				"method", info.FullMethod,
				"client_ip", clientIP,
				"duration_ms", duration.Milliseconds(),
				"error", err.Error(),
			)
		} else {
			logger.Info(ctx, "GRPC stream request completed",
				"method", info.FullMethod,
				"client_ip", clientIP,
				"duration_ms", duration.Milliseconds(),
			)
		}

		return err
	}
}

// wrappedServerStream 是一个包装了grpc.ServerStream的结构体，用于传递自定义上下文
type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

// Context 返回包装后的上下文
func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
