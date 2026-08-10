package grpcx

import (
	"context"
	"errors"
	"io"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/bosonicalcom/bedrock-go/syserr"
)

// -- Syserr client interceptor
// Converts gRPC status errors returned by outbound calls back into *syserr.Error,
// decoding the google.rpc error details the server attached to the status.

// SyserrClientInterceptor converts the gRPC status errors returned by outbound calls into
// [syserr.Error], restoring the structured details carried by the status. Callers can then
// inspect the result with [syserr.Is] and [syserr.IsDetailType] instead of unpacking the
// status themselves.
//
// The original error is preserved as a cause, so errors.Is and status.Code keep working.
func SyserrClientInterceptor() (grpc.UnaryClientInterceptor, grpc.StreamClientInterceptor) {
	unary := func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		return convertClientError(invoker(ctx, method, req, reply, cc, opts...))
	}

	stream := func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		cs, err := streamer(ctx, desc, cc, method, opts...)
		if err != nil {
			return nil, convertClientError(err)
		}
		return &syserrClientStream{ClientStream: cs}, nil
	}

	return unary, stream
}

// convertClientError converts a transport error into a [syserr.Error]. It leaves nil,
// io.EOF and errors that already are syserrs untouched.
func convertClientError(err error) error {
	// io.EOF signals a successful end of stream, never a failure.
	if err == nil || errors.Is(err, io.EOF) {
		return err
	}

	if _, ok := errors.AsType[*syserr.Error](err); ok {
		return err
	}

	if sysErr := ParseSystemError(err); sysErr != nil {
		return sysErr
	}

	return err
}

// syserrClientStream wraps a [grpc.ClientStream] to convert the errors it reports.
type syserrClientStream struct {
	grpc.ClientStream
}

// RecvMsg implements [grpc.ClientStream]. The terminal RPC status arrives here, so this is
// where a streaming call's syserr details are recovered.
func (s *syserrClientStream) RecvMsg(m any) error {
	return convertClientError(s.ClientStream.RecvMsg(m))
}

// SendMsg implements [grpc.ClientStream]. Only client-generated failures surface here; an
// io.EOF means the status must be read from RecvMsg instead, and is passed through.
func (s *syserrClientStream) SendMsg(m any) error {
	return convertClientError(s.ClientStream.SendMsg(m))
}

// Header implements [grpc.ClientStream]. It reports a status error when the stream fails
// before any header is received.
func (s *syserrClientStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	return md, convertClientError(err)
}
