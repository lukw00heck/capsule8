// Code generated by protoc-gen-grpc-gateway. DO NOT EDIT.
// source: capsule8/api/v0/telemetry_service.proto

/*
Package capsule8_api_v0 is a reverse proxy.

It translates gRPC into RESTful JSON APIs.
*/
package capsule8_api_v0

import (
	"io"
	"net/http"

	"github.com/golang/protobuf/proto"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/grpclog"
	"google.golang.org/grpc/status"
)

var _ codes.Code
var _ io.Reader
var _ status.Status
var _ = runtime.String
var _ = utilities.NewDoubleArray

func request_TelemetryService_GetEvents_0(ctx context.Context, marshaler runtime.Marshaler, client TelemetryServiceClient, req *http.Request, pathParams map[string]string) (TelemetryService_GetEventsClient, runtime.ServerMetadata, error) {
	var protoReq GetEventsRequest
	var metadata runtime.ServerMetadata

	if err := marshaler.NewDecoder(req.Body).Decode(&protoReq); err != nil {
		return nil, metadata, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	stream, err := client.GetEvents(ctx, &protoReq)
	if err != nil {
		return nil, metadata, err
	}
	header, err := stream.Header()
	if err != nil {
		return nil, metadata, err
	}
	metadata.HeaderMD = header
	return stream, metadata, nil

}

// RegisterTelemetryServiceHandlerFromEndpoint is same as RegisterTelemetryServiceHandler but
// automatically dials to "endpoint" and closes the connection when "ctx" gets done.
func RegisterTelemetryServiceHandlerFromEndpoint(ctx context.Context, mux *runtime.ServeMux, endpoint string, opts []grpc.DialOption) (err error) {
	conn, err := grpc.Dial(endpoint, opts...)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			if cerr := conn.Close(); cerr != nil {
				grpclog.Printf("Failed to close conn to %s: %v", endpoint, cerr)
			}
			return
		}
		go func() {
			<-ctx.Done()
			if cerr := conn.Close(); cerr != nil {
				grpclog.Printf("Failed to close conn to %s: %v", endpoint, cerr)
			}
		}()
	}()

	return RegisterTelemetryServiceHandler(ctx, mux, conn)
}

// RegisterTelemetryServiceHandler registers the http handlers for service TelemetryService to "mux".
// The handlers forward requests to the grpc endpoint over "conn".
func RegisterTelemetryServiceHandler(ctx context.Context, mux *runtime.ServeMux, conn *grpc.ClientConn) error {
	return RegisterTelemetryServiceHandlerClient(ctx, mux, NewTelemetryServiceClient(conn))
}

// RegisterTelemetryServiceHandler registers the http handlers for service TelemetryService to "mux".
// The handlers forward requests to the grpc endpoint over the given implementation of "TelemetryServiceClient".
// Note: the gRPC framework executes interceptors within the gRPC handler. If the passed in "TelemetryServiceClient"
// doesn't go through the normal gRPC flow (creating a gRPC client etc.) then it will be up to the passed in
// "TelemetryServiceClient" to call the correct interceptors.
func RegisterTelemetryServiceHandlerClient(ctx context.Context, mux *runtime.ServeMux, client TelemetryServiceClient) error {

	mux.Handle("POST", pattern_TelemetryService_GetEvents_0, func(w http.ResponseWriter, req *http.Request, pathParams map[string]string) {
		ctx, cancel := context.WithCancel(req.Context())
		defer cancel()
		if cn, ok := w.(http.CloseNotifier); ok {
			go func(done <-chan struct{}, closed <-chan bool) {
				select {
				case <-done:
				case <-closed:
					cancel()
				}
			}(ctx.Done(), cn.CloseNotify())
		}
		inboundMarshaler, outboundMarshaler := runtime.MarshalerForRequest(mux, req)
		rctx, err := runtime.AnnotateContext(ctx, mux, req)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}
		resp, md, err := request_TelemetryService_GetEvents_0(rctx, inboundMarshaler, client, req, pathParams)
		ctx = runtime.NewServerMetadataContext(ctx, md)
		if err != nil {
			runtime.HTTPError(ctx, mux, outboundMarshaler, w, req, err)
			return
		}

		forward_TelemetryService_GetEvents_0(ctx, mux, outboundMarshaler, w, req, func() (proto.Message, error) { return resp.Recv() }, mux.GetForwardResponseOptions()...)

	})

	return nil
}

var (
	pattern_TelemetryService_GetEvents_0 = runtime.MustPattern(runtime.NewPattern(1, []int{2, 0, 2, 1}, []string{"v0", "events"}, ""))
)

var (
	forward_TelemetryService_GetEvents_0 = runtime.ForwardResponseStream
)
