package grpcclient

import (
	"context"
	"errors"
	"testing"

	"github.com/sony/gobreaker/v2"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestUnaryInterceptorOpensAfterConsecutiveTransientFailures(t *testing.T) {
	settings := gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 2
		},
	}
	breaker := gobreaker.NewCircuitBreaker[any](settings)
	calls := 0
	interceptor := unaryInterceptor(breaker)
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		calls++
		return status.Error(codes.Unavailable, "dependency unavailable")
	}

	for i := 0; i < 2; i++ {
		err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
		require.Error(t, err)
	}

	err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	require.Error(t, err)
	require.ErrorIs(t, err, gobreaker.ErrOpenState)
	require.Equal(t, 2, calls)
}

func TestUnaryInterceptorDoesNotTripOnBusinessErrors(t *testing.T) {
	settings := gobreaker.Settings{
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures >= 1
		},
		IsSuccessful: func(err error) bool {
			return status.Code(err) != codes.Unavailable &&
				status.Code(err) != codes.DeadlineExceeded &&
				status.Code(err) != codes.ResourceExhausted
		},
	}
	breaker := gobreaker.NewCircuitBreaker[any](settings)
	businessErr := status.Error(codes.InvalidArgument, "invalid request")
	interceptor := unaryInterceptor(breaker)
	invoker := func(context.Context, string, any, any, *grpc.ClientConn, ...grpc.CallOption) error {
		return businessErr
	}

	for i := 0; i < 3; i++ {
		err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
		require.Equal(t, status.Code(businessErr), status.Code(err))
	}
}

func TestUnaryInterceptorPreservesExistingDeadline(t *testing.T) {
	breaker := gobreaker.NewCircuitBreaker[any](gobreaker.Settings{})
	interceptor := unaryInterceptor(breaker)
	expected := errors.New("failed")
	invoker := func(ctx context.Context, _ string, _, _ any, _ *grpc.ClientConn, _ ...grpc.CallOption) error {
		_, ok := ctx.Deadline()
		require.True(t, ok)
		return expected
	}

	err := interceptor(context.Background(), "/test.Service/Method", nil, nil, nil, invoker)
	require.ErrorIs(t, err, expected)
}
