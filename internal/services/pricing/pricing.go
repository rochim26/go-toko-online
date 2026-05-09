package pricing

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

// PriceFor returns the unit price for a variant given an audience.
// Audience is "b2c" or a tier UUID string. Falls back to base_price * (1 - tier.discount_pct).
func (s *Service) PriceFor(ctx context.Context, variantID uuid.UUID, audience string, tierDiscountPct float64) (float64, error) {
	now := time.Now()
	var price float64
	err := s.pool.QueryRow(ctx, `
		SELECT price FROM product_prices
		WHERE variant_id=$1 AND audience=$2
		  AND (valid_from IS NULL OR valid_from <= $3)
		  AND (valid_to   IS NULL OR valid_to   >= $3)
		ORDER BY valid_from DESC NULLS LAST LIMIT 1`,
		variantID, audience, now).Scan(&price)
	if err == nil {
		return price, nil
	}
	// Fallback to base_price minus tier discount
	var base float64
	if err := s.pool.QueryRow(ctx, `SELECT base_price FROM product_variants WHERE id=$1`, variantID).Scan(&base); err != nil {
		return 0, err
	}
	if tierDiscountPct > 0 {
		base = base * (100 - tierDiscountPct) / 100
	}
	return base, nil
}

// AudienceFor returns ("b2c", 0) for end users / no tier, or (tier_id_str, discount_pct) for resellers with tier.
type Audience struct {
	Code        string
	DiscountPct float64
	TierID      *uuid.UUID
	MoqQty      int
	MoqValue    float64
	TopDays     int
}

func (s *Service) AudienceForUser(ctx context.Context, userID *uuid.UUID) (Audience, error) {
	if userID == nil {
		return Audience{Code: "b2c"}, nil
	}
	var role string
	if err := s.pool.QueryRow(ctx, `SELECT role FROM users WHERE id=$1`, *userID).Scan(&role); err != nil {
		return Audience{Code: "b2c"}, err
	}
	if role != "reseller" {
		return Audience{Code: "b2c"}, nil
	}
	var tierID uuid.UUID
	var discount float64
	var moq int
	var moqValue float64
	var topDays int
	err := s.pool.QueryRow(ctx, `
		SELECT t.id, t.discount_pct, t.moq_qty, t.moq_value, t.top_days
		FROM reseller_profiles rp
		JOIN reseller_tiers t ON t.id = rp.tier_id
		WHERE rp.user_id=$1 AND rp.status='approved'`, *userID).
		Scan(&tierID, &discount, &moq, &moqValue, &topDays)
	if err != nil {
		return Audience{Code: "b2c"}, nil
	}
	return Audience{Code: tierID.String(), DiscountPct: discount, TierID: &tierID, MoqQty: moq, MoqValue: moqValue, TopDays: topDays}, nil
}
