package order

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tokoonline/app/internal/models"
)

type Service struct{ pool *pgxpool.Pool }

func New(pool *pgxpool.Pool) *Service { return &Service{pool: pool} }

type Address struct {
	Recipient  string
	Phone      string
	Address    string
	Province   string
	City       string
	District   string
	PostalCode string
	AreaID     string
}

type CreateInput struct {
	UserID         *uuid.UUID
	Channel        string
	PaymentTerm    string
	TopDays        int
	CustomerEmail  string
	CustomerName   string
	CustomerPhone  string
	Address        Address
	CourierCode    string
	CourierService string
	ShippingTotal  float64
	DiscountTotal  float64
	TaxTotal       float64
	VoucherCode    string
	Notes          string
	UTM            map[string]string
	FBP            string
	FBC            string
	ClientIP       string
	UserAgent      string
	Items          []OrderItemInput
	InvoicePrefix  string
}

type OrderItemInput struct {
	VariantID uuid.UUID
	SKU       string
	Name      string
	Qty       int
	UnitPrice float64
	ImageURL  string
	Attrs     map[string]any
}

func (s *Service) Create(ctx context.Context, in CreateInput) (*models.Order, error) {
	if len(in.Items) == 0 {
		return nil, errors.New("order must have at least 1 item")
	}
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	subtotal := 0.0
	for _, it := range in.Items {
		subtotal += it.UnitPrice * float64(it.Qty)
	}
	grand := subtotal - in.DiscountTotal + in.ShippingTotal + in.TaxTotal
	if grand < 0 {
		grand = 0
	}

	prefix := in.InvoicePrefix
	if prefix == "" {
		prefix = "INV"
	}
	code := fmt.Sprintf("%s-%s-%s", prefix, time.Now().Format("20060102"), strings.ToUpper(uuid.NewString()[:6]))

	utmJSON, _ := json.Marshal(in.UTM)

	var topDueAt *time.Time
	status := "awaiting_payment"
	paymentStatus := "unpaid"
	if in.PaymentTerm == "top" {
		t := time.Now().AddDate(0, 0, in.TopDays)
		topDueAt = &t
		status = "on_credit"
		paymentStatus = "top"
	}

	o := models.Order{}
	err = tx.QueryRow(ctx, `
		INSERT INTO orders(
			code, user_id, channel, status, payment_status, payment_term, top_due_at,
			subtotal, discount_total, shipping_total, tax_total, grand_total,
			ship_recipient, ship_phone, ship_address, ship_province, ship_city, ship_district, ship_postal_code, ship_area_id,
			courier_code, courier_service,
			customer_email, customer_phone, customer_name,
			utm, fbp, fbc, client_ip, user_agent,
			notes, voucher_code
		) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30,$31,$32)
		RETURNING id, code, status, payment_status, payment_term, top_due_at, subtotal, discount_total, shipping_total, tax_total, grand_total, created_at`,
		code, in.UserID, in.Channel, status, paymentStatus, in.PaymentTerm, topDueAt,
		subtotal, in.DiscountTotal, in.ShippingTotal, in.TaxTotal, grand,
		in.Address.Recipient, in.Address.Phone, in.Address.Address, in.Address.Province, in.Address.City, in.Address.District, in.Address.PostalCode, nullStr(in.Address.AreaID),
		nullStr(in.CourierCode), nullStr(in.CourierService),
		in.CustomerEmail, in.CustomerPhone, in.CustomerName,
		utmJSON, nullStr(in.FBP), nullStr(in.FBC), nullStr(in.ClientIP), nullStr(in.UserAgent),
		nullStr(in.Notes), nullStr(in.VoucherCode),
	).Scan(&o.ID, &o.Code, &o.Status, &o.PaymentStatus, &o.PaymentTerm, &o.TopDueAt, &o.Subtotal, &o.DiscountTotal, &o.ShippingTotal, &o.TaxTotal, &o.GrandTotal, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	o.Channel = in.Channel
	o.UserID = in.UserID

	for _, it := range in.Items {
		attrJSON, _ := json.Marshal(it.Attrs)
		_, err := tx.Exec(ctx, `INSERT INTO order_items(order_id,variant_id,sku,name,qty,unit_price,line_total,image_url,attributes)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			o.ID, it.VariantID, it.SKU, it.Name, it.Qty, it.UnitPrice, it.UnitPrice*float64(it.Qty), nullStr(it.ImageURL), attrJSON)
		if err != nil {
			return nil, err
		}
		// reserve inventory
		_, err = tx.Exec(ctx, `UPDATE inventory_levels SET reserved = reserved + $2 WHERE variant_id=$1`, it.VariantID, it.Qty)
		if err != nil {
			return nil, err
		}
		o.Items = append(o.Items, models.OrderItem{
			ID: uuid.New(), OrderID: o.ID, VariantID: &it.VariantID,
			SKU: it.SKU, Name: it.Name, Qty: it.Qty, UnitPrice: it.UnitPrice, LineTotal: it.UnitPrice * float64(it.Qty),
		})
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	o.CustomerEmail = &in.CustomerEmail
	o.CustomerName = &in.CustomerName
	return &o, nil
}

func (s *Service) AttachXenditInvoice(ctx context.Context, orderID uuid.UUID, invoiceID, invoiceURL string) error {
	_, err := s.pool.Exec(ctx, `UPDATE orders SET xendit_invoice_id=$2, xendit_invoice_url=$3, updated_at=now() WHERE id=$1`, orderID, invoiceID, invoiceURL)
	return err
}

func (s *Service) MarkPaid(ctx context.Context, externalID string, method string, paidAmount float64) (*models.Order, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	o := models.Order{}
	err = tx.QueryRow(ctx, `
		UPDATE orders SET payment_status='paid', payment_method=$2, paid_total=$3, paid_at=now(),
			status='paid', updated_at=now()
		WHERE code=$1 AND payment_status<>'paid'
		RETURNING id, code, user_id, status, payment_status, grand_total, customer_email, customer_phone, customer_name`,
		externalID, method, paidAmount).
		Scan(&o.ID, &o.Code, &o.UserID, &o.Status, &o.PaymentStatus, &o.GrandTotal, &o.CustomerEmail, &o.CustomerPhone, &o.CustomerName)
	if err != nil {
		// already paid
		return nil, err
	}
	// move reserved -> on_hand subtract
	rows, err := tx.Query(ctx, `SELECT variant_id, qty FROM order_items WHERE order_id=$1`, o.ID)
	if err != nil {
		return nil, err
	}
	type vq struct {
		v   uuid.UUID
		qty int
	}
	var vqs []vq
	for rows.Next() {
		var x vq
		var vid *uuid.UUID
		if err := rows.Scan(&vid, &x.qty); err != nil {
			rows.Close()
			return nil, err
		}
		if vid != nil {
			x.v = *vid
			vqs = append(vqs, x)
		}
	}
	rows.Close()
	for _, x := range vqs {
		if _, err := tx.Exec(ctx, `UPDATE inventory_levels SET on_hand=on_hand-$2, reserved=GREATEST(reserved-$2,0), updated_at=now() WHERE variant_id=$1`, x.v, x.qty); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return &o, nil
}

func (s *Service) GetByCode(ctx context.Context, code string) (*models.Order, error) {
	var o models.Order
	err := s.pool.QueryRow(ctx, `SELECT id, code, user_id, channel, status, payment_status, payment_method, payment_term, top_due_at,
		subtotal, discount_total, shipping_total, tax_total, grand_total, paid_total,
		ship_recipient, ship_phone, ship_address, ship_province, ship_city, ship_district, ship_postal_code, ship_area_id,
		courier_code, courier_service, awb,
		customer_email, customer_phone, customer_name,
		xendit_invoice_id, xendit_invoice_url, notes, voucher_code, paid_at, created_at
		FROM orders WHERE code=$1`, code).Scan(
		&o.ID, &o.Code, &o.UserID, &o.Channel, &o.Status, &o.PaymentStatus, &o.PaymentMethod, &o.PaymentTerm, &o.TopDueAt,
		&o.Subtotal, &o.DiscountTotal, &o.ShippingTotal, &o.TaxTotal, &o.GrandTotal, &o.PaidTotal,
		&o.ShipRecipient, &o.ShipPhone, &o.ShipAddress, &o.ShipProvince, &o.ShipCity, &o.ShipDistrict, &o.ShipPostalCode, &o.ShipAreaID,
		&o.CourierCode, &o.CourierService, &o.AWB,
		&o.CustomerEmail, &o.CustomerPhone, &o.CustomerName,
		&o.XenditInvoiceID, &o.XenditInvoiceURL, &o.Notes, &o.VoucherCode, &o.PaidAt, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id, order_id, variant_id, sku, name, qty, unit_price, line_total, image_url FROM order_items WHERE order_id=$1`, o.ID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var it models.OrderItem
		if err := rows.Scan(&it.ID, &it.OrderID, &it.VariantID, &it.SKU, &it.Name, &it.Qty, &it.UnitPrice, &it.LineTotal, &it.ImageURL); err != nil {
			return nil, err
		}
		o.Items = append(o.Items, it)
	}
	return &o, nil
}

func (s *Service) ListByUser(ctx context.Context, userID uuid.UUID) ([]*models.Order, error) {
	rows, err := s.pool.Query(ctx, `SELECT id, code, channel, status, payment_status, grand_total, created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []*models.Order{}
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.Code, &o.Channel, &o.Status, &o.PaymentStatus, &o.GrandTotal, &o.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, &o)
	}
	return out, rows.Err()
}

func (s *Service) AdminList(ctx context.Context, status, channel string, limit, offset int) ([]*models.Order, int, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	args := []any{limit, offset}
	where := []string{"1=1"}
	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("status=$%d", len(args)))
	}
	if channel != "" {
		args = append(args, channel)
		where = append(where, fmt.Sprintf("channel=$%d", len(args)))
	}
	sql := "SELECT id, code, channel, status, payment_status, grand_total, customer_name, customer_email, created_at FROM orders WHERE " + strings.Join(where, " AND ") + " ORDER BY created_at DESC LIMIT $1 OFFSET $2"
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []*models.Order{}
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.Code, &o.Channel, &o.Status, &o.PaymentStatus, &o.GrandTotal, &o.CustomerName, &o.CustomerEmail, &o.CreatedAt); err != nil {
			return nil, 0, err
		}
		out = append(out, &o)
	}
	cargs := args[2:]
	csql := "SELECT count(*) FROM orders WHERE " + strings.Join(where, " AND ")
	var total int
	if err := s.pool.QueryRow(ctx, csql, cargs...).Scan(&total); err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, status, awb, courierCode, courierService string) error {
	_, err := s.pool.Exec(ctx, `UPDATE orders SET status=$2, awb=COALESCE(NULLIF($3,''), awb), courier_code=COALESCE(NULLIF($4,''), courier_code), courier_service=COALESCE(NULLIF($5,''), courier_service), updated_at=now() WHERE id=$1`,
		id, status, awb, courierCode, courierService)
	return err
}

func nullStr(s string) any {
	if s == "" {
		return nil
	}
	return s
}
