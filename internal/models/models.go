package models

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID            uuid.UUID
	Email         string
	Phone         *string
	PasswordHash  string
	Role          string
	FullName      *string
	IsActive      bool
	EmailVerified bool
	LastLoginAt   *time.Time
	CreatedAt     time.Time
}

type ResellerTier struct {
	ID          uuid.UUID
	Code        string
	Name        string
	DiscountPct float64
	MoqQty      int
	MoqValue    float64
	CreditLimit float64
	TopDays     int
	SortOrder   int
	IsActive    bool
}

type ResellerProfile struct {
	UserID                uuid.UUID
	TierID                *uuid.UUID
	StoreName             string
	NPWP                  *string
	KTPNumber             *string
	Address               *string
	Province              *string
	City                  *string
	District              *string
	PostalCode            *string
	ContactPhone          *string
	Status                string
	RejectionReason       *string
	CreditLimitOverride   *float64
	Notes                 *string
	ApprovedAt            *time.Time
}

type Brand struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	LogoURL     *string
	Description *string
}

type Category struct {
	ID          uuid.UUID
	ParentID    *uuid.UUID
	Name        string
	Slug        string
	Description *string
	ImageURL    *string
	SortOrder   int
	IsActive    bool
	SeoTitle    *string
	SeoDesc     *string
	GMCCategory *string
}

type Product struct {
	ID            uuid.UUID
	Slug          string
	Name          string
	BrandID       *uuid.UUID
	CategoryID    *uuid.UUID
	ShortDesc     *string
	Description   *string
	Status        string
	IsB2C         bool
	IsB2B         bool
	WeightGrams   int
	LengthCm      *int
	WidthCm       *int
	HeightCm      *int
	SeoTitle      *string
	SeoDesc       *string
	FocusKeyword  *string
	OGImageURL    *string
	GMCEnabled    bool
	GMCBrand      *string
	GMCGtin       *string
	GMCMpn        *string
	GMCCondition  string
	GMCAgeGroup   *string
	GMCGender     *string
	FAQs          []FAQ
	PublishedAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// joined
	BrandName    *string
	BrandSlug    *string
	CategoryName *string
	CategorySlug *string
	PrimaryImage *string
	MinPrice     float64
	MaxPrice     float64
	OnHand       int
}

type FAQ struct {
	Q string `json:"q"`
	A string `json:"a"`
}

type Variant struct {
	ID              uuid.UUID
	ProductID       uuid.UUID
	SKU             string
	Name            *string
	Attributes      map[string]any
	BasePrice       float64
	CompareAtPrice  *float64
	CostPrice       *float64
	WeightGrams     *int
	Barcode         *string
	IsDefault       bool
	IsActive        bool
}

type ProductImage struct {
	ID        uuid.UUID
	ProductID uuid.UUID
	VariantID *uuid.UUID
	URL       string
	AltText   *string
	SortOrder int
	IsPrimary bool
}

type CartItem struct {
	ID        uuid.UUID
	CartID    uuid.UUID
	VariantID uuid.UUID
	Qty       int
	UnitPrice float64
	// joined
	ProductSlug string
	ProductName string
	VariantSKU  string
	VariantName *string
	ImageURL    *string
	WeightGrams int
	OnHand      int
}

type Cart struct {
	ID           uuid.UUID
	UserID       *uuid.UUID
	SessionToken *string
	Channel      string
	Audience     string
	Notes        *string
	Items        []CartItem
}

type OrderItem struct {
	ID        uuid.UUID
	OrderID   uuid.UUID
	VariantID *uuid.UUID
	SKU       string
	Name      string
	Qty       int
	UnitPrice float64
	LineTotal float64
	ImageURL  *string
}

type Order struct {
	ID                 uuid.UUID
	Code               string
	UserID             *uuid.UUID
	Channel            string
	Status             string
	PaymentStatus      string
	PaymentMethod      *string
	PaymentTerm        string
	TopDueAt           *time.Time
	Subtotal           float64
	DiscountTotal      float64
	ShippingTotal      float64
	TaxTotal           float64
	GrandTotal         float64
	PaidTotal          float64
	ShipRecipient      *string
	ShipPhone          *string
	ShipAddress        *string
	ShipProvince       *string
	ShipCity           *string
	ShipDistrict       *string
	ShipPostalCode     *string
	ShipAreaID         *string
	CourierCode        *string
	CourierService     *string
	AWB                *string
	CustomerEmail      *string
	CustomerPhone      *string
	CustomerName       *string
	XenditInvoiceID    *string
	XenditInvoiceURL   *string
	Notes              *string
	VoucherCode        *string
	PaidAt             *time.Time
	CreatedAt          time.Time
	Items              []OrderItem
}

type CustomerAddress struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Label      string
	Recipient  string
	Phone      string
	Address    string
	Province   string
	City       string
	District   *string
	PostalCode string
	AreaID     *string
	IsDefault  bool
	CreatedAt  time.Time
}

type Page struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	BodyHTML    string
	SeoTitle    *string
	SeoDesc     *string
	IsPublished bool
	UpdatedAt   time.Time
}

type BlogPost struct {
	ID          uuid.UUID
	Slug        string
	Title       string
	Excerpt     *string
	BodyHTML    string
	CoverURL    *string
	SeoTitle    *string
	SeoDesc     *string
	Author      *string
	IsPublished bool
	PublishedAt *time.Time
	UpdatedAt   time.Time
}

type Voucher struct {
	ID          uuid.UUID
	Code        string
	Name        *string
	Type        string
	Value       float64
	MinSubtotal float64
	MaxDiscount *float64
	Audience    string
	UsageLimit  *int
	UsedCount   int
	ValidFrom   *time.Time
	ValidTo     *time.Time
	IsActive    bool
}
