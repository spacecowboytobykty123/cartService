package postgres

import (
	"cartService/internal/data"
	"cartService/internal/validator"
	"context"
	"database/sql"
	"errors"
	_ "github.com/lib/pq"
	cart_v1_crt "github.com/spacecowboytobykty123/protoCart/proto/gen/go/cart"
	"time"
)

type Storage struct {
	db *sql.DB
}

const (
	emptyValue = 0
)

type StorageDetails struct {
	DSN          string
	MaxOpenConns int
	MaxIdleConns int
	MaxIdleTime  string
}

func OpenDB(details StorageDetails) (*Storage, error) {
	db, err := sql.Open("postgres", details.DSN)

	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(details.MaxOpenConns)
	db.SetMaxIdleConns(details.MaxIdleConns)

	duration, err := time.ParseDuration(details.MaxIdleTime)

	db.SetConnMaxIdleTime(duration)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}
	return &Storage{
		db: db,
	}, err
}

func ValidateToy(v *validator.Validator, toy data.CartItem) {
	v.Check(toy.ToyID != emptyValue, "text", "toy id must be provided")
	v.Check(toy.Quantity != emptyValue, "text", "quantity must be provided")
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) AddToCart(ctx context.Context, toy data.CartItem, userID int64) (cart_v1_crt.OperationStatus, string) {
	query := `INSERT INTO cart_items (user_id, toy_id, quantity)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, toy_id)
DO UPDATE SET
  quantity = cart_items.quantity + EXCLUDED.quantity,
  updated_at = NOW()
RETURNING id;
`

	query1 := `INSERT INTO carts (user_id)
VALUES ($1)
ON CONFLICT (user_id) DO NOTHING;`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err := s.db.ExecContext(ctx, query1, userID)
	if err != nil {
		return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to get user cart"
	}

	var itemID int64
	args := []any{userID, toy.ToyID, toy.Quantity}
	println("db part")

	err = s.db.QueryRowContext(ctx, query, args...).Scan(&itemID)
	if err != nil {
		println(err.Error())
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to add toy"
		default:
			return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to add toy"
		}
	}

	return cart_v1_crt.OperationStatus_STATUS_OK, "Toy added to a cart!"
}

func (s *Storage) DelFromCart(ctx context.Context, toyId int64, userID int64) (cart_v1_crt.OperationStatus, string) {
	query := `DELETE FROM cart_items
WHERE user_id = $1 AND toy_id = $2
`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	results, err := s.db.ExecContext(ctx, query, userID, toyId)
	if err != nil {
		return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to delete toy!"
	}
	rowsAffected, err := results.RowsAffected()
	if err != nil {
		return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to delete toy!"
	}

	if rowsAffected == 0 {
		return cart_v1_crt.OperationStatus_STATUS_INTERNAL_ERROR, "failed to delete toy!"
	}
	return cart_v1_crt.OperationStatus_STATUS_OK, "deleted successfully"

}

func (s *Storage) GetCart(ctx context.Context, userID int64) ([]*data.CartItem, int32, int32) {
	query := `SELECT toy_id, quantity from cart_items
WHERE user_id = $1
`

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		println("1")
		println(err.Error())
		return []*data.CartItem{}, 0, 0
	}
	defer rows.Close()

	toys := []*data.CartItem{}

	for rows.Next() {
		var toy data.CartItem

		err := rows.Scan(
			&toy.ToyID,
			&toy.Quantity,
		)

		if err != nil {
			println("1")
			println(err.Error())
			return []*data.CartItem{}, 0, 0
		}

		toys = append(toys, &toy)
	}

	if err = rows.Err(); err != nil {
		println("3")
		println(err.Error())
		return []*data.CartItem{}, 0, 0
	}

	var totalqty, totalToys int32

	err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) from cart_items WHERE user_id = $1`, userID).Scan(&totalToys)
	if err != nil {
		println("totalToys db part")
		return []*data.CartItem{}, 0, 0
	}

	err = s.db.QueryRowContext(ctx, `SELECT SUM(quantity) FROM cart_items WHERE user_id=$1`, userID).Scan(&totalqty)
	if err != nil {
		println("totalToys qty db part")
		return []*data.CartItem{}, 0, 0
	}
	return toys, totalToys, totalqty
}
