package app

import (
	"context"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/scs/postgresstore"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/justinas/nosurf"

	"github.com/tokoonline/app/internal/config"
	"github.com/tokoonline/app/internal/handlers/admin"
	apih "github.com/tokoonline/app/internal/handlers/api"
	publich "github.com/tokoonline/app/internal/handlers/public"
	resh "github.com/tokoonline/app/internal/handlers/reseller"
	"github.com/tokoonline/app/internal/middleware"
	"github.com/tokoonline/app/internal/services/auth"
	"github.com/tokoonline/app/internal/services/cart"
	"github.com/tokoonline/app/internal/services/catalog"
	"github.com/tokoonline/app/internal/services/cron"
	"github.com/tokoonline/app/internal/services/gmc"
	"github.com/tokoonline/app/internal/services/mailer"
	"github.com/tokoonline/app/internal/services/order"
	"github.com/tokoonline/app/internal/services/payment"
	"github.com/tokoonline/app/internal/services/pricing"
	"github.com/tokoonline/app/internal/services/reseller"
	"github.com/tokoonline/app/internal/services/settings"
	"github.com/tokoonline/app/internal/services/shipping"
	"github.com/tokoonline/app/internal/services/tracking"
)

type App struct {
	Cfg      *config.Config
	Pool     *pgxpool.Pool
	DB       *sql.DB
	Sess     *scs.SessionManager
	Settings *settings.Store
	Auth     *auth.Service
	Catalog  *catalog.Service
	Pricing  *pricing.Service
	Cart     *cart.Service
	Order    *order.Service
	Reseller *reseller.Service
	Xendit   *payment.Xendit
	Biteship *shipping.Biteship
	Tracking *tracking.Service
	GMC      *gmc.Service
	Mailer   *mailer.Mailer
	Cron     *cron.Runner
}

func New(cfg *config.Config, pool *pgxpool.Pool, db *sql.DB) *App {
	st := settings.New(pool)
	_ = st.Reload(context.Background())
	a := &App{Cfg: cfg, Pool: pool, DB: db, Settings: st}

	sess := scs.New()
	sess.Lifetime = 30 * 24 * time.Hour
	sess.IdleTimeout = 14 * 24 * time.Hour
	sess.Cookie.Name = "mdt_session"
	sess.Cookie.Path = "/"
	sess.Cookie.HttpOnly = true
	sess.Cookie.SameSite = http.SameSiteLaxMode
	sess.Cookie.Secure = strings.HasPrefix(cfg.BaseURL, "https://")
	sess.Store = postgresstore.New(db)
	a.Sess = sess

	a.Auth = auth.New(pool)
	a.Catalog = catalog.New(pool)
	a.Pricing = pricing.New(pool)
	a.Cart = cart.New(pool, a.Pricing)
	a.Order = order.New(pool)
	a.Reseller = reseller.New(pool)
	a.Xendit = payment.New(st)
	a.Biteship = shipping.New(st)
	a.Tracking = tracking.New(st)
	a.GMC = gmc.New(pool, st)
	a.Mailer = mailer.New(st)
	a.Cron = &cron.Runner{Pool: pool, Mailer: a.Mailer, Settings: st, Tracking: a.Tracking, BaseURL: cfg.BaseURL}
	a.Cron.Start(context.Background())

	go func() {
		t := time.NewTicker(30 * time.Second)
		for range t.C {
			_ = st.Reload(context.Background())
		}
	}()

	return a
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Compress(5))
	r.Use(middleware.SecurityHeaders)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	r.Handle("/uploads/*", http.StripPrefix("/uploads/", http.FileServer(http.Dir(a.Cfg.UploadDir))))
	r.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/manifest.webmanifest")
	})
	r.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Service-Worker-Allowed", "/")
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, "static/sw.js")
	})
	r.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/img/favicon.ico")
	})
	r.HandleFunc("/apple-touch-icon.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/img/apple-touch-icon.png")
	})
	r.HandleFunc("/apple-touch-icon-precomposed.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "static/img/apple-touch-icon.png")
	})

	publicH := &publich.Handler{
		Pool: a.Pool, Settings: a.Settings, Catalog: a.Catalog,
		Cart: a.Cart, Pricing: a.Pricing, Order: a.Order, BaseURL: a.Cfg.BaseURL,
	}
	checkoutH := &publich.CheckoutHandler{
		Pool: a.Pool, Settings: a.Settings, Cart: a.Cart, Pricing: a.Pricing,
		Order: a.Order, Xendit: a.Xendit, Biteship: a.Biteship, Tracking: a.Tracking,
		Mailer: a.Mailer,
		BaseURL: a.Cfg.BaseURL, Sessions: a.Sess, Public: publicH,
	}
	authH := &publich.AuthHandler{Pool: a.Pool, Auth: a.Auth, Cart: a.Cart, Mailer: a.Mailer, Sessions: a.Sess, Public: publicH}
	accountH := &publich.AccountHandler{Pool: a.Pool, Cart: a.Cart, Pricing: a.Pricing, Order: a.Order, Sessions: a.Sess, Public: publicH}
	adminH := &admin.Handler{
		Pool: a.Pool, Auth: a.Auth, Catalog: a.Catalog, Order: a.Order, Reseller: a.Reseller,
		Settings: a.Settings, Sessions: a.Sess, UploadDir: a.Cfg.UploadDir, BaseURL: a.Cfg.BaseURL,
		Mailer: a.Mailer,
	}
	resellerH := &resh.Handler{
		Pool: a.Pool, Auth: a.Auth, Reseller: a.Reseller, Catalog: a.Catalog, Cart: a.Cart,
		Pricing: a.Pricing, Settings: a.Settings, Sessions: a.Sess, BaseURL: a.Cfg.BaseURL,
		Mailer: a.Mailer, UploadDir: a.Cfg.UploadDir,
	}
	gmcH := &apih.GMCHandler{GMC: a.GMC, BaseURL: a.Cfg.BaseURL}

	r.Group(func(r chi.Router) {
		r.Use(a.Sess.LoadAndSave)
		r.Use(middleware.InjectSessionToken(a.Sess))
		r.Use(csrfMiddleware(strings.HasPrefix(a.Cfg.BaseURL, "https://")))
		r.Use(publicH.RedirectMiddleware)

		// Public
		r.Get("/", publicH.Home)
		r.Get("/c/{slug}", publicH.CatalogPage)
		r.Get("/p/{slug}", publicH.Product)
		r.Get("/search", publicH.Search)
		r.Get("/cart", publicH.CartPage)
		r.Get("/checkout", checkoutH.Show)
		r.Post("/checkout", checkoutH.Submit)
		r.Get("/order/success", checkoutH.OrderSuccessPage)
		r.Get("/order/failed", checkoutH.OrderFailedPage)
		r.Get("/blog", publicH.BlogIndex)
		r.Get("/blog/{slug}", publicH.BlogPost)
		r.Get("/robots.txt", publicH.Robots)
		r.Get("/sitemap.xml", publicH.Sitemap)
		r.Get("/feeds/gmc.xml", gmcH.Feed)
		r.Get("/page/{slug}", publicH.Page)
		r.Get("/panduan", publicH.Guides)

		// API
		r.Post("/api/cart/add", publicH.APICartAdd)
		r.Post("/api/cart/update", publicH.APICartUpdate)
		r.Post("/api/cart/remove", publicH.APICartRemove)
		r.Get("/api/shipping/areas", checkoutH.AreasSearch)
		r.Get("/api/shipping/rates", checkoutH.Rates)
		r.Post("/api/attribution", publicH.APIAttribution)

		// Webhooks
		r.Post("/webhooks/xendit", checkoutH.XenditWebhook)

		// Auth
		r.Get("/login", authH.ShowLogin)
		r.Post("/login", authH.Login)
		r.Get("/register", authH.ShowRegister)
		r.Post("/register", authH.Register)
		r.Get("/logout", authH.Logout)

		// Customer area
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("customer", "reseller", "admin", "staff"))
			r.Get("/account", accountH.Home)
			r.Get("/account/orders", authH.OrderHistory)
			r.Get("/account/orders/{code}", checkoutH.OrderShow)
			r.Get("/account/orders/{code}/po.pdf", adminH.OrderPOPDF)
			r.Post("/account/orders/{code}/reorder", accountH.Reorder)
			r.Get("/account/password", authH.ShowChangePassword)
			r.Post("/account/password", authH.ChangePassword)
			r.Get("/account/profile", accountH.ShowProfile)
			r.Post("/account/profile", accountH.SaveProfile)
			r.Get("/account/addresses", accountH.Addresses)
			r.Post("/account/addresses", accountH.AddressCreate)
			r.Get("/account/addresses/{id}/edit", accountH.AddressEdit)
			r.Post("/account/addresses/{id}", accountH.AddressUpdate)
			r.Post("/account/addresses/{id}/delete", accountH.AddressDelete)
			r.Post("/account/addresses/{id}/default", accountH.AddressMakeDefault)
		})

		// Admin
		r.Get("/admin/login", adminH.ShowLogin)
		r.Post("/admin/login", adminH.Login)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("admin", "staff"))
			r.Get("/admin", adminH.Dashboard)
			r.Get("/admin/products", adminH.Products)
			r.Get("/admin/products/new", adminH.NewProductForm)
			r.Post("/admin/products", adminH.CreateProduct)
			r.Get("/admin/products/{id}", adminH.EditProduct)
			r.Post("/admin/products/{id}", adminH.UpdateProduct)
			r.Post("/admin/products/{id}/variants", adminH.AddVariant)
			r.Post("/admin/products/{id}/variants/{vid}/delete", adminH.DeleteVariant)
			r.Post("/admin/products/{id}/variants/{vid}/prices", adminH.SetVariantPrice)
			r.Post("/admin/products/{id}/variants/{vid}/inventory", adminH.SetInventory)
			r.Post("/admin/products/{id}/images", adminH.UploadImage)
			r.Post("/admin/products/{id}/images/{iid}/delete", adminH.DeleteImage)
			r.Get("/admin/orders", adminH.Orders)
			r.Get("/admin/orders/{code}", adminH.OrderDetail)
			r.Post("/admin/orders/{code}/status", adminH.UpdateOrderStatus)
			r.Get("/admin/orders/{code}/po.pdf", adminH.OrderPOPDF)
			r.Get("/admin/resellers", adminH.ResellersList)
			r.Post("/admin/resellers/{id}/approve", adminH.ResellerApprove)
			r.Post("/admin/resellers/{id}/reject", adminH.ResellerReject)
			r.Get("/admin/tiers", adminH.Tiers)
			r.Post("/admin/tiers", adminH.TierUpsert)
			r.Get("/admin/settings", adminH.SettingsPage)
			r.Post("/admin/settings/{tab}", adminH.SaveSettings)
			r.Post("/admin/upload", adminH.GenericUpload)
			r.Get("/admin/password", adminH.ShowChangePassword)
			r.Post("/admin/password", adminH.ChangePassword)
			r.Get("/admin/vouchers", adminH.Vouchers)
			r.Post("/admin/vouchers", adminH.VoucherUpsert)
			r.Post("/admin/vouchers/{id}/toggle", adminH.VoucherToggle)
			r.Get("/admin/categories", adminH.Categories)
			r.Post("/admin/categories", adminH.CategoryCreate)
			r.Post("/admin/categories/{id}/toggle", adminH.CategoryToggle)
			r.Post("/admin/categories/{id}/delete", adminH.CategoryDelete)
			r.Get("/admin/blog", adminH.BlogList)
			r.Get("/admin/blog/new", adminH.BlogNew)
			r.Post("/admin/blog", adminH.BlogCreate)
			r.Get("/admin/blog/{id}", adminH.BlogEdit)
			r.Post("/admin/blog/{id}", adminH.BlogUpdate)
			// Sesi 4: missing pages + tour + test connections
			r.Get("/admin/customers", adminH.Customers)
			r.Get("/admin/pages", adminH.Pages)
			r.Post("/admin/pages", adminH.PageUpsert)
			r.Post("/admin/pages/{id}/delete", adminH.PageDelete)
			r.Get("/admin/redirects", adminH.Redirects)
			r.Post("/admin/redirects", adminH.RedirectCreate)
			r.Post("/admin/redirects/{id}/delete", adminH.RedirectDelete)
			r.Get("/admin/feeds", adminH.FeedStatus)
			r.Get("/admin/sitemap", adminH.SitemapStatus)
			r.Get("/admin/tour", adminH.TourPage)
			r.Post("/admin/tour/start", adminH.ResetTour)
			r.Post("/admin/onboarding/complete", adminH.CompleteOnboarding)
			r.Get("/admin/test-connection/{provider}", adminH.TestConnection)
		})

		// Reseller
		r.Get("/reseller/login", resellerH.ShowLogin)
		r.Post("/reseller/login", resellerH.Login)
		r.Get("/reseller/register", resellerH.ShowRegister)
		r.Post("/reseller/register", resellerH.Register)
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireRole("reseller"))
			r.Get("/reseller", resellerH.Dashboard)
			r.Get("/reseller/catalog", resellerH.CatalogPage)
			r.Get("/reseller/bulk", resellerH.BulkPage)
			r.Post("/reseller/bulk/add", resellerH.BulkAdd)
			r.Post("/reseller/bulk/csv", resellerH.BulkCSV)
			r.Get("/reseller/cart", publicH.CartPage)
			r.Get("/reseller/profile", resellerH.Profile)
			r.Get("/reseller/statements", resellerH.Statements)
			r.Get("/reseller/orders", authH.OrderHistory)
		})
	})

	return r
}

// csrfMiddleware returns a middleware that wraps each request through a single nosurf instance.
// nosurf is stateless between requests (the cookie is the source of truth), so wrapping `next`
// inline is fine, but we expose failure messages for debugging.
func csrfMiddleware(secure bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		cs := nosurf.New(next)
		cs.SetBaseCookie(http.Cookie{
			Name:     "csrf_token",
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Secure:   secure,
			MaxAge:   86400,
		})
		cs.ExemptRegexp(`^/webhooks/`)
		cs.ExemptRegexp(`^/feeds/`)
		cs.ExemptRegexp(`^/sitemap\.xml$`)
		cs.ExemptRegexp(`^/robots\.txt$`)
		cs.ExemptRegexp(`^/api/attribution$`)
		cs.SetFailureHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			reason := nosurf.Reason(r)
			msg := "csrf check failed"
			if reason != nil {
				msg = "csrf: " + reason.Error()
			}
			http.Error(w, msg, http.StatusForbidden)
		}))
		return cs
	}
}
