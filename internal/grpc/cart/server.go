package cart

import (
	"cartService/internal/data"
	"cartService/internal/validator"
	"cartService/storage/postgres"
	"context"
	"fmt"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/gen/go/cart"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"strings"
)

type serverAPI struct {
	cart_v1_crt.UnimplementedCartServer
	carts Carts
}

type Carts interface {
	AddToCart(ctx context.Context, toy data.CartItem) (cart_v1_crt.OperationStatus, string)
	DelFromCart(ctx context.Context, toyId int64) (cart_v1_crt.OperationStatus, string)
	GetCart(ctx context.Context) ([]*data.CartItem, int32, int32)
}

func Register(gRPC *grpc.Server, carts Carts) {
	cart_v1_crt.RegisterCartServer(gRPC, &serverAPI{carts: carts})
}

const (
	emptyValue = 0
)

func (s *serverAPI) AddToCart(ctx context.Context, r *cart_v1_crt.AddToCartRequest) (*cart_v1_crt.AddToCartResponse, error) {
	v := validator.New()

	toy := r.GetToy()
	inputToy := data.CartItem{
		ToyID:    toy.ToyId,
		Quantity: toy.Quantity,
	}

	if postgres.ValidateToy(v, inputToy); !v.Valid() {
		return nil, collectErrors(v)
	}

	opStatus, msg := s.carts.AddToCart(ctx, inputToy)

	return &cart_v1_crt.AddToCartResponse{
		OpStatus: opStatus,
		Message:  msg,
	}, nil
}

func (s *serverAPI) DelFromCart(ctx context.Context, r *cart_v1_crt.DelFromCartRequest) (*cart_v1_crt.DelFromCartResponse, error) {
	toyID := r.GetToyId()

	if toyID == emptyValue {
		return nil, nil
	}

	opStatus, msg := s.carts.DelFromCart(ctx, toyID)
	return &cart_v1_crt.DelFromCartResponse{
		OpStatus: opStatus,
		Message:  msg,
	}, nil
}

func (s *serverAPI) GetCart(ctx context.Context, r *cart_v1_crt.GetCartRequest) (*cart_v1_crt.GetCartResponse, error) {
	toys, total_items, total_qty := s.carts.GetCart(ctx)

	return &cart_v1_crt.GetCartResponse{
		Items:         ToDomainOrder(toys),
		TotalItems:    total_items,
		TotalQuantity: total_qty,
	}, nil
}

func collectErrors(v *validator.Validator) error {
	var b strings.Builder
	for field, msg := range v.Errors {
		fmt.Fprintf(&b, "%s:%s; ", field, msg)
	}
	return status.Error(codes.InvalidArgument, b.String())
}

func ToDomainOrder(toys []*data.CartItem) []*cart_v1_crt.CartItem {
	domainToys := make([]*cart_v1_crt.CartItem, 0, len(toys))
	for _, o := range toys {
		domainToys = append(domainToys, &cart_v1_crt.CartItem{
			ToyId:    o.ToyID,
			Quantity: o.Quantity,
		})
	}
	return domainToys
}
