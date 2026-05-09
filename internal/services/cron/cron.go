package cron

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/services/tracking"
)

type Runner struct {
	Pool     *pgxpool.Pool
	Mailer   *mailer.Mailer
	Settings *settings.Store
	Tracking *tracking.Service
	BaseURL  string
}

// Start runs background jobs forever.
func (r *Runner) Start(ctx context.Context) {
	go r.loop(ctx, "abandoned-cart", 30*time.Minute, r.AbandonedCart)
	go r.loop(ctx, "top-overdue", 6*time.Hour, r.MarkOverdueTOP)
}

func (r *Runner) loop(ctx context.Context, name string, every time.Duration, fn func(context.Context) error) {
	t := time.NewTicker(every)
	defer t.Stop()
	// run once at startup, but staggered
	go func() {
		time.Sleep(2 * time.Minute)
		if err := fn(ctx); err != nil {
			log.Printf("cron[%s] startup error: %v", name, err)
		}
	}()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := fn(ctx); err != nil {
				log.Printf("cron[%s] error: %v", name, err)
			}
		}
	}
}

// AbandonedCart finds carts older than 4h that have not been notified yet,
// where the user has an email, and sends a reminder.
func (r *Runner) AbandonedCart(ctx context.Context) error {
	rows, err := r.Pool.Query(ctx, `
		SELECT c.id, COALESCE(u.email,''), COALESCE(u.full_name,'')
		FROM carts c
		LEFT JOIN users u ON u.id = c.user_id
		WHERE c.user_id IS NOT NULL
		  AND COALESCE(u.email,'') <> ''
		  AND c.abandoned_notified = FALSE
		  AND c.updated_at < now() - interval '4 hours'
		  AND c.updated_at > now() - interval '7 days'
		  AND EXISTS (SELECT 1 FROM cart_items ci WHERE ci.cart_id = c.id)
		LIMIT 100`)
	if err != nil {
		return err
	}
	defer rows.Close()
	type cand struct{ id, email, name string }
	var cands []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.email, &c.name); err == nil {
			cands = append(cands, c)
		}
	}
	rows.Close()
	if len(cands) == 0 {
		return nil
	}
	store := r.Settings.Store()
	for _, c := range cands {
		// load items
		ir, err := r.Pool.Query(ctx, `
			SELECT ci.id, ci.cart_id, ci.variant_id, ci.qty, ci.unit_price,
				p.slug, p.name, v.sku, v.name,
				(SELECT url FROM product_images WHERE product_id=p.id ORDER BY is_primary DESC LIMIT 1),
				COALESCE(v.weight_grams, p.weight_grams), 0
			FROM cart_items ci
			JOIN product_variants v ON v.id=ci.variant_id
			JOIN products p ON p.id=v.product_id
			WHERE ci.cart_id=$1`, c.id)
		if err != nil {
			continue
		}
		var items []models.CartItem
		for ir.Next() {
			var it models.CartItem
			if err := ir.Scan(&it.ID, &it.CartID, &it.VariantID, &it.Qty, &it.UnitPrice,
				&it.ProductSlug, &it.ProductName, &it.VariantSKU, &it.VariantName, &it.ImageURL,
				&it.WeightGrams, &it.OnHand); err == nil {
				items = append(items, it)
			}
		}
		ir.Close()
		if len(items) == 0 {
			continue
		}
		subj, body := mailer.AbandonedCart(store.Name, r.BaseURL, c.name, items)
		_ = r.Mailer.Send(ctx, c.email, subj, body)
		_, _ = r.Pool.Exec(ctx, `UPDATE carts SET abandoned_notified=TRUE WHERE id=$1`, c.id)
	}
	return nil
}

// MarkOverdueTOP flags TOP orders past due as overdue.
func (r *Runner) MarkOverdueTOP(ctx context.Context) error {
	_, err := r.Pool.Exec(ctx, `
		UPDATE orders SET status='overdue', updated_at=now()
		WHERE payment_term='top' AND payment_status<>'paid'
		  AND top_due_at IS NOT NULL AND top_due_at < now()
		  AND status NOT IN ('overdue','cancelled','refunded')`)
	return err
}
