package cart

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/pricing"
)

type Service struct {
	pool    *pgxpool.Pool
	pricing *pricing.Service
}

func New(pool *pgxpool.Pool, pr *pricing.Service) *Service {
	return &Service{pool: pool, pricing: pr}
}

// GetOrCreate returns the open cart for the given session/user.
func (s *Service) GetOrCreate(ctx context.Context, sessionToken string, userID *uuid.UUID, audience string) (*models.Cart, error) {
	channel := "b2c"
	if audience != "b2c" {
		channel = "b2b"
	}
	var c models.Cart
	var err error
	if userID != nil {
		err = s.pool.QueryRow(ctx, `SELECT id,user_id,session_token,channel,audience,notes FROM carts WHERE user_id=$1 ORDER BY created_at DESC LIMIT 1`, *userID).
			Scan(&c.ID, &c.UserID, &c.SessionToken, &c.Channel, &c.Audience, &c.Notes)
	} else {
		err = s.pool.QueryRow(ctx, `SELECT id,user_id,session_token,channel,audience,notes FROM carts WHERE session_token=$1 ORDER BY created_at DESC LIMIT 1`, sessionToken).
			Scan(&c.ID, &c.UserID, &c.SessionToken, &c.Channel, &c.Audience, &c.Notes)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `INSERT INTO carts(user_id,session_token,channel,audience) VALUES($1,$2,$3,$4) RETURNING id,user_id,session_token,channel,audience,notes`,
			userID, sessionToken, channel, audience).
			Scan(&c.ID, &c.UserID, &c.SessionToken, &c.Channel, &c.Audience, &c.Notes)
	}
	if err != nil {
		return nil, err
	}
	if c.Audience != audience {
		_, _ = s.pool.Exec(ctx, `UPDATE carts SET audience=$2, channel=$3, updated_at=now() WHERE id=$1`, c.ID, audience, channel)
		c.Audience = audience
		c.Channel = channel
	}
	c.Items, err = s.items(ctx, c.ID)
	return &c, err
}

func (s *Service) items(ctx context.Context, cartID uuid.UUID) ([]models.CartItem, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT ci.id, ci.cart_id, ci.variant_id, ci.qty, ci.unit_price,
			p.slug, p.name, v.sku, v.name,
			(SELECT url FROM product_images WHERE product_id=p.id ORDER BY is_primary DESC, sort_order LIMIT 1),
			COALESCE(v.weight_grams, p.weight_grams),
			COALESCE((SELECT on_hand-reserved FROM inventory_levels il WHERE il.variant_id=v.id),0)
		FROM cart_items ci
		JOIN product_variants v ON v.id = ci.variant_id
		JOIN products p ON p.id = v.product_id
		WHERE ci.cart_id=$1
		ORDER BY ci.created_at`, cartID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CartItem{}
	for rows.Next() {
		var it models.CartItem
		if err := rows.Scan(&it.ID, &it.CartID, &it.VariantID, &it.Qty, &it.UnitPrice,
			&it.ProductSlug, &it.ProductName, &it.VariantSKU, &it.VariantName, &it.ImageURL, &it.WeightGrams, &it.OnHand); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Service) Add(ctx context.Context, c *models.Cart, variantID uuid.UUID, qty int, audience string, discountPct float64) error {
	if qty <= 0 {
		qty = 1
	}
	price, err := s.pricing.PriceFor(ctx, variantID, audience, discountPct)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO cart_items(cart_id,variant_id,qty,unit_price) VALUES($1,$2,$3,$4)
		ON CONFLICT(cart_id,variant_id) DO UPDATE SET qty=cart_items.qty+EXCLUDED.qty, unit_price=EXCLUDED.unit_price`, c.ID, variantID, qty, price)
	if err != nil {
		return err
	}
	_, _ = s.pool.Exec(ctx, `UPDATE carts SET updated_at=now() WHERE id=$1`, c.ID)
	return nil
}

func (s *Service) UpdateQty(ctx context.Context, cartID, itemID uuid.UUID, qty int) error {
	if qty <= 0 {
		_, err := s.pool.Exec(ctx, `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`, itemID, cartID)
		return err
	}
	_, err := s.pool.Exec(ctx, `UPDATE cart_items SET qty=$3 WHERE id=$1 AND cart_id=$2`, itemID, cartID, qty)
	return err
}

func (s *Service) Remove(ctx context.Context, cartID, itemID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM cart_items WHERE id=$1 AND cart_id=$2`, itemID, cartID)
	return err
}

func (s *Service) Clear(ctx context.Context, cartID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM cart_items WHERE cart_id=$1`, cartID)
	return err
}

// MergeOnLogin attaches an anonymous cart to a user (called after login).
func (s *Service) MergeOnLogin(ctx context.Context, sessionToken string, userID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE carts SET user_id=$2, updated_at=now() WHERE session_token=$1 AND user_id IS NULL`, sessionToken, userID)
	return err
}

// Subtotal returns the items subtotal.
func Subtotal(c *models.Cart) float64 {
	var t float64
	for _, it := range c.Items {
		t += it.UnitPrice * float64(it.Qty)
	}
	return t
}

// TotalQty returns the total quantity across items.
func TotalQty(c *models.Cart) int {
	var n int
	for _, it := range c.Items {
		n += it.Qty
	}
	return n
}

// TotalWeightGrams returns the weight estimate.
func TotalWeightGrams(c *models.Cart) int {
	var n int
	for _, it := range c.Items {
		n += it.WeightGrams * it.Qty
	}
	if n < 100 {
		n = 100
	}
	return n
}
