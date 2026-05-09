package reseller

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/models"
	"github.com/tokoonline/app/internal/services/security"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type RegisterInput struct {
	Email        string
	Password     string
	FullName     string
	StoreName    string
	Phone        string
	NPWP         string
	KTPNumber    string
	Address      string
	Province     string
	City         string
	District     string
	PostalCode   string
	Docs         []string // urls of uploaded docs
	AutoApprove  bool
	DefaultTier  *uuid.UUID
}

func (s *Service) Register(ctx context.Context, in RegisterInput) (uuid.UUID, error) {
	hash, err := security.HashPassword(in.Password)
	if err != nil {
		return uuid.Nil, err
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return uuid.Nil, err
	}
	defer tx.Rollback(ctx)
	var uid uuid.UUID
	if err := tx.QueryRow(ctx, `INSERT INTO users(email,password_hash,role,full_name,phone) VALUES($1,$2,'reseller',$3,$4) RETURNING id`,
		in.Email, hash, nullStr(in.FullName), nullStr(in.Phone)).Scan(&uid); err != nil {
		return uuid.Nil, err
	}
	docs, _ := json.Marshal(in.Docs)
	status := "pending"
	if in.AutoApprove {
		status = "approved"
	}
	_, err = tx.Exec(ctx, `INSERT INTO reseller_profiles(user_id,tier_id,store_name,npwp,ktp_number,address,province,city,district,postal_code,contact_phone,docs,status,approved_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13, CASE WHEN $13='approved' THEN now() ELSE NULL END)`,
		uid, in.DefaultTier, in.StoreName, nullStr(in.NPWP), nullStr(in.KTPNumber), nullStr(in.Address), nullStr(in.Province), nullStr(in.City), nullStr(in.District), nullStr(in.PostalCode), nullStr(in.Phone), docs, status)
	if err != nil {
		return uuid.Nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return uuid.Nil, err
	}
	return uid, nil
}

func (s *Service) ListTiers(ctx context.Context) ([]*models.ResellerTier, error) {
	rows, err := s.pool.Query(ctx, `SELECT id,code,name,discount_pct,moq_qty,moq_value,credit_limit,top_days,sort_order,is_active FROM reseller_tiers ORDER BY sort_order, name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.ResellerTier{}
	for rows.Next() {
		var t models.ResellerTier
		if err := rows.Scan(&t.ID, &t.Code, &t.Name, &t.DiscountPct, &t.MoqQty, &t.MoqValue, &t.CreditLimit, &t.TopDays, &t.SortOrder, &t.IsActive); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}

func (s *Service) Approve(ctx context.Context, userID uuid.UUID, tierID uuid.UUID, approverID uuid.UUID) error {
	_, err := s.pool.Exec(ctx, `UPDATE reseller_profiles SET status='approved', tier_id=$2, approved_at=now(), approved_by=$3, updated_at=now() WHERE user_id=$1`, userID, tierID, approverID)
	return err
}

func (s *Service) Reject(ctx context.Context, userID uuid.UUID, reason string) error {
	_, err := s.pool.Exec(ctx, `UPDATE reseller_profiles SET status='rejected', rejection_reason=$2, updated_at=now() WHERE user_id=$1`, userID, reason)
	return err
}

type ResellerListItem struct {
	UserID    uuid.UUID
	Email     string
	StoreName string
	Status    string
	TierName  *string
	CreatedAt string
}

func (s *Service) AdminList(ctx context.Context, status string) ([]*ResellerListItem, error) {
	q := `SELECT u.id, u.email, rp.store_name, rp.status, t.name, to_char(rp.created_at, 'YYYY-MM-DD HH24:MI')
		FROM reseller_profiles rp
		JOIN users u ON u.id = rp.user_id
		LEFT JOIN reseller_tiers t ON t.id = rp.tier_id`
	args := []any{}
	if status != "" {
		q += " WHERE rp.status=$1"
		args = append(args, status)
	}
	q += " ORDER BY rp.created_at DESC LIMIT 200"
	rows, err := s.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*ResellerListItem{}
	for rows.Next() {
		it := &ResellerListItem{}
		if err := rows.Scan(&it.UserID, &it.Email, &it.StoreName, &it.Status, &it.TierName, &it.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Service) Profile(ctx context.Context, userID uuid.UUID) (*models.ResellerProfile, *models.ResellerTier, error) {
	p := &models.ResellerProfile{UserID: userID}
	var status string
	err := s.pool.QueryRow(ctx, `SELECT tier_id, store_name, npwp, ktp_number, address, province, city, district, postal_code, contact_phone, status, rejection_reason, credit_limit_override, notes, approved_at FROM reseller_profiles WHERE user_id=$1`, userID).
		Scan(&p.TierID, &p.StoreName, &p.NPWP, &p.KTPNumber, &p.Address, &p.Province, &p.City, &p.District, &p.PostalCode, &p.ContactPhone, &status, &p.RejectionReason, &p.CreditLimitOverride, &p.Notes, &p.ApprovedAt)
	if err != nil {
		return nil, nil, err
	}
	p.Status = status
	if p.TierID == nil {
		return p, nil, nil
	}
	t := &models.ResellerTier{}
	err = s.pool.QueryRow(ctx, `SELECT id,code,name,discount_pct,moq_qty,moq_value,credit_limit,top_days,sort_order,is_active FROM reseller_tiers WHERE id=$1`, *p.TierID).
		Scan(&t.ID, &t.Code, &t.Name, &t.DiscountPct, &t.MoqQty, &t.MoqValue, &t.CreditLimit, &t.TopDays, &t.SortOrder, &t.IsActive)
	if errors.Is(err, pgx.ErrNoRows) {
		return p, nil, nil
	}
	return p, t, err
}

type Statement struct {
	Month       string
	OrderCount  int
	GrandTotal  float64
	UnpaidCount int
	UnpaidTotal float64
}

func (s *Service) MonthlyStatements(ctx context.Context, userID uuid.UUID) ([]*Statement, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT to_char(date_trunc('month',created_at),'YYYY-MM'),
		       count(*),
		       COALESCE(sum(grand_total),0),
		       COALESCE(sum(CASE WHEN payment_status<>'paid' THEN 1 ELSE 0 END),0),
		       COALESCE(sum(CASE WHEN payment_status<>'paid' THEN grand_total ELSE 0 END),0)
		FROM orders WHERE user_id=$1
		GROUP BY 1 ORDER BY 1 DESC LIMIT 12`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*Statement{}
	for rows.Next() {
		var st Statement
		if err := rows.Scan(&st.Month, &st.OrderCount, &st.GrandTotal, &st.UnpaidCount, &st.UnpaidTotal); err != nil {
			return nil, err
		}
		out = append(out, &st)
	}
	return out, rows.Err()
}

// CreditUsage returns the current outstanding TOP balance.
func (s *Service) CreditUsage(ctx context.Context, userID uuid.UUID) (float64, error) {
	var v float64
	err := s.pool.QueryRow(ctx, `SELECT COALESCE(sum(grand_total - paid_total),0) FROM orders WHERE user_id=$1 AND payment_term='top' AND payment_status<>'paid'`, userID).Scan(&v)
	return v, err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
