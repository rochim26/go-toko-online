package auth

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/services/security"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

var ErrInvalid = errors.New("email atau password salah")
var ErrInactive = errors.New("akun belum aktif atau ditolak")

type AuthedUser struct {
	ID    uuid.UUID
	Email string
	Role  string
	Name  string
}

func (s *Service) Authenticate(ctx context.Context, email, password string) (*AuthedUser, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	var id uuid.UUID
	var hash, role string
	var fullName *string
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT id, password_hash, role, full_name, is_active FROM users WHERE email=$1`, email).
		Scan(&id, &hash, &role, &fullName, &active)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrInvalid
	}
	if err != nil {
		return nil, err
	}
	ok, err := security.VerifyPassword(password, hash)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrInvalid
	}
	if !active {
		return nil, ErrInactive
	}
	if role == "reseller" {
		var status string
		_ = s.pool.QueryRow(ctx, `SELECT status FROM reseller_profiles WHERE user_id=$1`, id).Scan(&status)
		if status != "approved" {
			return nil, ErrInactive
		}
	}
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, id)
	name := ""
	if fullName != nil {
		name = *fullName
	}
	return &AuthedUser{ID: id, Email: email, Role: role, Name: name}, nil
}

func (s *Service) RegisterCustomer(ctx context.Context, email, password, fullName, phone string) (uuid.UUID, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	hash, err := security.HashPassword(password)
	if err != nil {
		return uuid.Nil, err
	}
	var id uuid.UUID
	err = s.pool.QueryRow(ctx, `INSERT INTO users(email,password_hash,role,full_name,phone) VALUES($1,$2,'customer',$3,$4) RETURNING id`,
		email, hash, nullStr(fullName), nullStr(phone)).Scan(&id)
	return id, err
}

func (s *Service) EnsureAdmin(ctx context.Context, email, password string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if password == "" {
		return errors.New("admin password required")
	}
	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE email=$1`, email).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		hash, err := security.HashPassword(password)
		if err != nil {
			return err
		}
		_, err = s.pool.Exec(ctx, `INSERT INTO users(email,password_hash,role,full_name,is_active,email_verified) VALUES($1,$2,'admin','Admin',TRUE,TRUE)`, email, hash)
		return err
	}
	return err
}

// Touch updates last login
func (s *Service) Touch(ctx context.Context, id uuid.UUID) {
	_, _ = s.pool.Exec(ctx, `UPDATE users SET last_login_at=now() WHERE id=$1`, id)
}

// Now is exposed for tests
var Now = func() time.Time { return time.Now() }

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
