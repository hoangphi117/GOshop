package grpcclient

import (
	"context"
	"fmt"
	"time"

	"goshop/pkg/telemetry"

	"github.com/sony/gobreaker/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultRPCTimeout = 5 * time.Second
	breakerFailures   = 5
	breakerTimeout    = 30 * time.Second
)

// NewClient creates a gRPC connection with a bounded timeout and circuit breaker.
func NewClient(target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	settings := gobreaker.Settings{
		Name:        fmt.Sprintf("grpc:%s", target),
		MaxRequests: 1,
		Interval:    0,
		Timeout:     breakerTimeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= breakerFailures
		},
		IsSuccessful: func(err error) bool {
			code := status.Code(err)
			return code != codes.Unavailable &&
				code != codes.DeadlineExceeded &&
				code != codes.ResourceExhausted
		},
	}
	breaker := gobreaker.NewCircuitBreaker[any](settings)

	opts = append(opts, grpc.WithChainUnaryInterceptor(telemetry.UnaryClientInterceptor(), unaryInterceptor(breaker)))
	return grpc.NewClient(target, opts...)
}

func unaryInterceptor(breaker *gobreaker.CircuitBreaker[any]) grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		request, reply any,
		conn *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultRPCTimeout)
			defer cancel()
		}

		_, err := breaker.Execute(func() (any, error) {
			return nil, invoker(ctx, method, request, reply, conn, opts...)
		})
		return err
	}
}
