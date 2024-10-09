package logger

import (
	"context"
	"log/slog"
)

type ctxKey string

const (
	slogFields ctxKey = "slog_fields"
)

type ContextHandler struct {
	slog.Handler
}

// todo look into adding this trace 


// Handle adds contextual attributes to the Record before calling the underlying handler
func (h ContextHandler) Handle(ctx context.Context, r slog.Record) error {
	if attrs, ok := ctx.Value(slogFields).([]slog.Attr); ok {
		for _, v := range attrs {
			r.AddAttrs(v)
		}
	}

	return h.Handler.Handle(ctx, r)
}

// AppendCtx adds an slog attribute to the provided context so that it will be
// included in any Record created with such context
func AppendCtx(parent context.Context, attr ...slog.Attr) context.Context {
	if parent == nil {
		parent = context.Background()
	}

	var newAttrs []slog.Attr
	if v, ok := parent.Value(slogFields).([]slog.Attr); ok {
		newAttrs = append(v, attr...)
	} else {
		newAttrs = append([]slog.Attr{}, attr...)
	}

	return context.WithValue(parent, slogFields, newAttrs)
}


// func ContextPropagationUnaryServerInterceptor() grpc.UnaryServerInterceptor {
// 	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
// 		// Get the metadata from the incoming context
// 		md, ok := metadata.FromIncomingContext(ctx)
// 		if !ok {
// 			return nil, fmt.Errorf("couldn't parse incoming context metadata")
// 		}

// 		for k, v := range md {
// 			if len(v) > 1 {
// 				ctx = AppendCtx(ctx, slog.Any(k, v))
// 			} else {
// 				ctx = AppendCtx(ctx, slog.String(k, v[0]))
// 			}

// 		}
// 		slog.InfoContext(ctx, "gRPC request")
// 		return handler(ctx, req)
// 	}
// }
