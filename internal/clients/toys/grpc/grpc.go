package grpc

import (
	"cartService/internal/jsonlog"
	"context"
	"fmt"
	grpclog "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/logging"
	grpcretry "github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/retry"
	"github.com/spacecowboytobykty123/toysProto/gen/go/toys"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"time"
)

type ToyClient struct {
	toyApi toys.ToysClient
	log    *jsonlog.Logger
}

func New(ctx context.Context, log *jsonlog.Logger, timeout time.Duration, retriesCount int) (*ToyClient, error) {

	retryOpts := []grpcretry.CallOption{
		grpcretry.WithCodes(codes.NotFound, codes.Aborted, codes.DeadlineExceeded),
		grpcretry.WithMax(uint(retriesCount)),
		grpcretry.WithPerRetryTimeout(timeout),
	}

	logOpts := []grpclog.Option{
		grpclog.WithLogOnEvents(grpclog.PayloadSent, grpclog.PayloadReceived),
	}

	cc, err := grpc.DialContext(ctx, "localhost:9000",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(
			grpclog.UnaryClientInterceptor(InterceptorLogger(log), logOpts...),
			grpcretry.UnaryClientInterceptor(retryOpts...),
		),
	)

	if err != nil {
		return nil, fmt.Errorf("%s:%w", "grpc.New", err)
	}
	return &ToyClient{
		toyApi: toys.NewToysClient(cc),
		log:    log,
	}, nil
}

func InterceptorLogger(logger *jsonlog.Logger) grpclog.Logger {
	return grpclog.LoggerFunc(func(ctx context.Context, lvl grpclog.Level, msg string, fields ...any) {
		logger.PrintInfo(msg, map[string]string{
			"lvl": string(lvl),
		})
	},
	)
}

func (t *ToyClient) GetToy(ctx context.Context, toyID int64) *toys.GetToyResponse {
	t.log.PrintInfo("getting toy from toy microservice", map[string]string{
		"method":  "toys.grpc.GetToy",
		"service": "Toys",
	})

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		t.log.PrintError(fmt.Errorf("missing metadata"), map[string]string{
			"method":  "toys.grpc.GetToy",
			"service": "Toys",
		})
		return &toys.GetToyResponse{
			Status: toys.Status_STATUS_INTERNAL_ERROR,
			Msg:    "missing metadata to connect to toys microservice",
		}
	}

	authHeader := md.Get("authorization")
	if len(authHeader) == 0 {
		t.log.PrintError(fmt.Errorf("missing authorization token"), nil)
		return &toys.GetToyResponse{
			Status: toys.Status_STATUS_INTERNAL_ERROR,
			Msg:    "missing auth token to connect to toys microservice",
		}
	}

	outctx := metadata.NewOutgoingContext(ctx, metadata.Pairs("authorization", authHeader[0]))
	t.log.PrintInfo("forwarding JWT token", map[string]string{
		"token": authHeader[0],
	})

	resp, err := t.toyApi.GetToy(outctx, &toys.GetToyRequest{ToyId: toyID})
	if err != nil {
		t.log.PrintError(fmt.Errorf("cannot get response from toy service"), map[string]string{
			"method": "toys.grpc.GetToy",
		})
		return &toys.GetToyResponse{Status: toys.Status_STATUS_INTERNAL_ERROR}
	}

	return resp
}
