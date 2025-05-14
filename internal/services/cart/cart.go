package cart

import (
	subsgrpc "cartService/internal/clients/subscriptions/grpc"
	"cartService/internal/contextkeys"
	"cartService/internal/data"
	"cartService/internal/jsonlog"
	"context"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/gen/go/cart"
	subs "github.com/spacecowboytobykty123/subsProto/gen/go/subscription"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"
)

type Carts struct {
	log          *jsonlog.Logger
	cartProvider cartProvider
	tokenTTL     time.Duration
	subsClient   *subsgrpc.Client
}

type cartProvider interface {
	AddToCart(ctx context.Context, toy data.CartItem, userID int64) (cart_v1_crt.OperationStatus, string)
	DelFromCart(ctx context.Context, toyId int64, userID int64) (cart_v1_crt.OperationStatus, string)
	GetCart(ctx context.Context, userID int64) ([]*data.CartItem, int32, int32)
}

func New(log *jsonlog.Logger, cartProvider cartProvider, tokenTTL time.Duration, subsClient *subsgrpc.Client) *Carts {
	return &Carts{
		log:          log,
		cartProvider: cartProvider,
		tokenTTL:     tokenTTL,
		subsClient:   subsClient,
	}
}

func (c Carts) AddToCart(ctx context.Context, toy data.CartItem) (cart_v1_crt.OperationStatus, string) {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return cart_v1_crt.OperationStatus_STATUS_INVALID_USER, "invalid user"
	}

	subsResp := c.subsClient.CheckSubscription(ctx, userID)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		return cart_v1_crt.OperationStatus_STATUS_INVALID_USER, "user is not subscribed!"
	}

	opStatus, msg := c.cartProvider.AddToCart(ctx, toy, userID)
	if opStatus != cart_v1_crt.OperationStatus_STATUS_OK {
		// TODO: лог добавить
		return opStatus, msg
	}

	return opStatus, msg
}

func (c Carts) DelFromCart(ctx context.Context, toyId int64) (cart_v1_crt.OperationStatus, string) {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		return cart_v1_crt.OperationStatus_STATUS_INVALID_USER, "invalid user"
	}

	subsResp := c.subsClient.CheckSubscription(ctx, userID)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		return cart_v1_crt.OperationStatus_STATUS_INVALID_USER, "user is not subscribed!"
	}
	opStatus, msg := c.cartProvider.DelFromCart(ctx, toyId, userID)
	if opStatus != cart_v1_crt.OperationStatus_STATUS_OK {
		// TODO: лог добавить
		return opStatus, msg
	}

	return opStatus, msg
}

func (c Carts) GetCart(ctx context.Context) ([]*data.CartItem, int32, int32) {
	userID, err := getUserFromContext(ctx)
	if err != nil {
		c.log.PrintError(status.Error(codes.Unauthenticated, "failed to authenticate user"), map[string]string{
			"method": "cart.GetCart",
		})
		return []*data.CartItem{}, 0, 0
	}

	subsResp := c.subsClient.CheckSubscription(ctx, userID)
	if subsResp.SubStatus != subs.Status_STATUS_SUBSCRIBED {
		c.log.PrintError(status.Error(codes.PermissionDenied, "user is not subscribed"), map[string]string{
			"method": "cart.GetCart",
		})
		return []*data.CartItem{}, 0, 0
	}

	toys, total_items, qty := c.cartProvider.GetCart(ctx, userID)
	if toys == nil {
		c.log.PrintError(status.Error(codes.NotFound, "failed to fetch toys"), map[string]string{
			"method": "cart.getCart",
		})
		return nil, 0, 0
	}

	return toys, total_items, qty
}

func getUserFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(contextkeys.UserIDKey)
	userID, ok := val.(int64)
	if !ok {
		return 0, status.Error(codes.Unauthenticated, "user id is missing or invalid in context")
	}

	return userID, nil

}
