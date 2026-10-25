package routes

import (
	_ "goshop/docs"
	v1routes "goshop/internal/routes/v1"
	"goshop/internal/utils"
	"goshop/pkg/auth"
	"goshop/pkg/cache"
	"goshop/pkg/middleware"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Route interface {
	Register(r *gin.RouterGroup)
}

func RegisterRoutes(r *gin.Engine, authService auth.TokenService, cacheService cache.RedisCacheService, normalRoutes []Route, adminRoutes []Route) {

	httpLogger := utils.NewLoggerWithPath("http.log", "info")
	recoveryLogger := utils.NewLoggerWithPath("recovery.log", "warning")
	rateLimiterLogger := utils.NewLoggerWithPath("rate_limiter.log", "warning")

	r.Use(gzip.Gzip(gzip.DefaultCompression))
	r.Use(
		middleware.RateLimiterMiddleware(rateLimiterLogger),
		middleware.CORSMiddleware(),
		middleware.TraceMiddleware(),
		middleware.LoggerMiddleware(httpLogger),
		middleware.RecoveryMiddleware(recoveryLogger),
	)

	middleware.InitAuthMiddleware(authService, cacheService)

	// Swagger documentation route
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/swagger", func(ctx *gin.Context) {
		ctx.Redirect(301, "/swagger/index.html")
	})

	// JWKS (JSON Web Key Set) endpoint for offline JWT verification across microservices
	jwksHandler := func(ctx *gin.Context) {
		ctx.Header("Cache-Control", "public, max-age=3600")
		ctx.JSON(http.StatusOK, authService.GetJWKS())
	}
	r.GET("/.well-known/jwks.json", jwksHandler)

	v1api := r.Group("/api/v1")
	v1api.GET("/.well-known/jwks.json", jwksHandler)

	// API Gateway reverse proxy for Identity microservice endpoints
	identityURL := utils.GetEnv("IDENTITY_SERVICE_URL", "http://localhost:8081")
	if parsedIdentityURL, err := url.Parse(identityURL); err == nil && parsedIdentityURL.Scheme != "" {
		identityProxy := httputil.NewSingleHostReverseProxy(parsedIdentityURL)
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableCompression = true
		identityProxy.Transport = transport

		identityProxy.ModifyResponse = func(resp *http.Response) error {
			// Strip upstream CORS headers to prevent duplicate values when combined with Gateway CORS middleware
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Max-Age")
			return nil
		}
		identityProxyHandler := func(ctx *gin.Context) {
			ctx.Request.Host = parsedIdentityURL.Host
			// Strip Accept-Encoding to prevent upstream double gzip compression
			ctx.Request.Header.Del("Accept-Encoding")
			identityProxy.ServeHTTP(ctx.Writer, ctx.Request)
		}

		v1api.Any("/auth", identityProxyHandler)
		v1api.Any("/auth/*any", identityProxyHandler)
		v1api.Any("/users", identityProxyHandler)
		v1api.Any("/users/*any", identityProxyHandler)
		v1api.Any("/addresses", identityProxyHandler)
		v1api.Any("/addresses/*any", identityProxyHandler)
		v1api.Any("/admin/users", identityProxyHandler)
		v1api.Any("/admin/users/*any", identityProxyHandler)
	}

	// API Gateway reverse proxy for Notification microservice endpoints
	notificationURL := utils.GetEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8082")
	if parsedNotifURL, err := url.Parse(notificationURL); err == nil && parsedNotifURL.Scheme != "" {
		notifProxy := httputil.NewSingleHostReverseProxy(parsedNotifURL)
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableCompression = true
		notifProxy.Transport = transport

		notifProxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Max-Age")
			return nil
		}
		notifProxyHandler := func(ctx *gin.Context) {
			ctx.Request.Host = parsedNotifURL.Host
			ctx.Request.Header.Del("Accept-Encoding")
			notifProxy.ServeHTTP(ctx.Writer, ctx.Request)
		}

		v1api.Any("/notifications", notifProxyHandler)
		v1api.Any("/notifications/*any", notifProxyHandler)
		v1api.Any("/admin/notifications", notifProxyHandler)
		v1api.Any("/admin/notifications/*any", notifProxyHandler)
	}

	// API Gateway reverse proxy for Payment microservice endpoints
	paymentURL := utils.GetEnv("PAYMENT_SERVICE_URL", "http://localhost:8083")
	if parsedPaymentURL, err := url.Parse(paymentURL); err == nil && parsedPaymentURL.Scheme != "" {
		paymentProxy := httputil.NewSingleHostReverseProxy(parsedPaymentURL)
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.DisableCompression = true
		paymentProxy.Transport = transport

		paymentProxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Del("Access-Control-Allow-Origin")
			resp.Header.Del("Access-Control-Allow-Credentials")
			resp.Header.Del("Access-Control-Allow-Methods")
			resp.Header.Del("Access-Control-Allow-Headers")
			resp.Header.Del("Access-Control-Max-Age")
			return nil
		}
		paymentProxyHandler := func(ctx *gin.Context) {
			ctx.Request.Host = parsedPaymentURL.Host
			ctx.Request.Header.Del("Accept-Encoding")
			paymentProxy.ServeHTTP(ctx.Writer, ctx.Request)
		}

		v1api.Any("/payments", paymentProxyHandler)
		v1api.Any("/payments/*any", paymentProxyHandler)
		v1api.Any("/admin/payments", paymentProxyHandler)
		v1api.Any("/admin/payments/*any", paymentProxyHandler)
		v1api.Any("/webhooks/payment", paymentProxyHandler)
		v1api.Any("/webhooks/payment/*any", paymentProxyHandler)
	}

	// API Gateway reverse proxy for Coupon microservice endpoints
	couponURL := utils.GetEnv("COUPON_SERVICE_URL", "")
	if couponURL != "" {
		if parsedCouponURL, err := url.Parse(couponURL); err == nil && parsedCouponURL.Scheme != "" {
			couponProxy := httputil.NewSingleHostReverseProxy(parsedCouponURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			couponProxy.Transport = transport

			couponProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			couponProxyHandler := func(ctx *gin.Context) {
				ctx.Request.Host = parsedCouponURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				couponProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/coupons", couponProxyHandler)
			v1api.Any("/coupons/*any", couponProxyHandler)
			v1api.Any("/admin/coupons", couponProxyHandler)
			v1api.Any("/admin/coupons/*any", couponProxyHandler)
		}
	}

	// API Gateway reverse proxy for Inventory microservice endpoints
	inventoryURL := utils.GetEnv("INVENTORY_SERVICE_URL", "")
	if inventoryURL != "" {
		if parsedInventoryURL, err := url.Parse(inventoryURL); err == nil && parsedInventoryURL.Scheme != "" {
			inventoryProxy := httputil.NewSingleHostReverseProxy(parsedInventoryURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			inventoryProxy.Transport = transport

			inventoryProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			inventoryProxyHandler := func(ctx *gin.Context) {
				ctx.Request.Host = parsedInventoryURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				inventoryProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/admin/inventory", inventoryProxyHandler)
			v1api.Any("/admin/inventory/*any", inventoryProxyHandler)
			v1api.Any("/inventory", inventoryProxyHandler)
			v1api.Any("/inventory/*any", inventoryProxyHandler)
		}
	}

	// API Gateway reverse proxy for Catalog microservice endpoints
	catalogURL := utils.GetEnv("CATALOG_SERVICE_URL", "http://localhost:8086")
	isCatalogProxied := false
	var catalogProxyHandler func(ctx *gin.Context)
	if catalogURL != "" {
		if parsedCatalogURL, err := url.Parse(catalogURL); err == nil && parsedCatalogURL.Scheme != "" {
			catalogProxy := httputil.NewSingleHostReverseProxy(parsedCatalogURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			catalogProxy.Transport = transport

			catalogProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			catalogProxyHandler = func(ctx *gin.Context) {
				ctx.Request.Host = parsedCatalogURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				catalogProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			// Proxy categories & admin endpoints directly (no sub-route conflicts)
			v1api.Any("/categories", catalogProxyHandler)
			v1api.Any("/categories/*any", catalogProxyHandler)
			v1api.Any("/admin/products", catalogProxyHandler)
			v1api.Any("/admin/products/*any", catalogProxyHandler)
			v1api.Any("/admin/categories", catalogProxyHandler)
			v1api.Any("/admin/categories/*any", catalogProxyHandler)
			// Public /products is routed in NoRoute so the catalog proxy can fall through
	// to the review service for /products/{uuid}/reviews*.
			isCatalogProxied = true
		}
	}

	// API Gateway reverse proxy for Cart microservice endpoints
	cartURL := utils.GetEnv("CART_SERVICE_URL", "")
	if cartURL != "" {
		if parsedCartURL, err := url.Parse(cartURL); err == nil && parsedCartURL.Scheme != "" {
			cartProxy := httputil.NewSingleHostReverseProxy(parsedCartURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			cartProxy.Transport = transport

			cartProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			cartProxyHandler := func(ctx *gin.Context) {
				ctx.Request.Host = parsedCartURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				cartProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/cart", cartProxyHandler)
			v1api.Any("/cart/*any", cartProxyHandler)
		}
	}

	// API Gateway reverse proxy for Banner microservice endpoints.
	// Dual-run: when BANNER_SERVICE_URL is unset, the in-monolith banner module
	// keeps serving /banners and /admin/banners (registered further below).
	bannerURL := utils.GetEnv("BANNER_SERVICE_URL", "")
	if bannerURL != "" {
		if parsedBannerURL, err := url.Parse(bannerURL); err == nil && parsedBannerURL.Scheme != "" {
			bannerProxy := httputil.NewSingleHostReverseProxy(parsedBannerURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			bannerProxy.Transport = transport

			bannerProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			bannerProxyHandler := func(ctx *gin.Context) {
				ctx.Request.Host = parsedBannerURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				bannerProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/banners", bannerProxyHandler)
			v1api.Any("/admin/banners", bannerProxyHandler)
			v1api.Any("/admin/banners/*any", bannerProxyHandler)
		}
	}

	// API Gateway reverse proxy for Order microservice endpoints
	orderURL := utils.GetEnv("ORDER_SERVICE_URL", "")
	if orderURL != "" {
		if parsedOrderURL, err := url.Parse(orderURL); err == nil && parsedOrderURL.Scheme != "" {
			orderProxy := httputil.NewSingleHostReverseProxy(parsedOrderURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			orderProxy.Transport = transport

			orderProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			orderProxyHandler := func(ctx *gin.Context) {
				ctx.Request.Host = parsedOrderURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				orderProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/orders", orderProxyHandler)
			v1api.Any("/orders/*any", orderProxyHandler)
			v1api.Any("/admin/orders", orderProxyHandler)
			v1api.Any("/admin/orders/*any", orderProxyHandler)
			v1api.Any("/admin/returns", orderProxyHandler)
			v1api.Any("/admin/returns/*any", orderProxyHandler)
			v1api.Any("/shipping", orderProxyHandler)
			v1api.Any("/shipping/*any", orderProxyHandler)
		}
	}

	// API Gateway reverse proxy for Review microservice endpoints. The monolith
	// no longer serves reviews: with REVIEW_SERVICE_URL unset these paths 404.
	reviewURL := utils.GetEnv("REVIEW_SERVICE_URL", "")
	isReviewProxied := false
	var reviewProxyHandler func(ctx *gin.Context)
	if reviewURL != "" {
		if parsedReviewURL, err := url.Parse(reviewURL); err == nil && parsedReviewURL.Scheme != "" {
			reviewProxy := httputil.NewSingleHostReverseProxy(parsedReviewURL)
			transport := http.DefaultTransport.(*http.Transport).Clone()
			transport.DisableCompression = true
			reviewProxy.Transport = transport

			reviewProxy.ModifyResponse = func(resp *http.Response) error {
				resp.Header.Del("Access-Control-Allow-Origin")
				resp.Header.Del("Access-Control-Allow-Credentials")
				resp.Header.Del("Access-Control-Allow-Methods")
				resp.Header.Del("Access-Control-Allow-Headers")
				resp.Header.Del("Access-Control-Max-Age")
				return nil
			}
			reviewProxyHandler = func(ctx *gin.Context) {
				ctx.Request.Host = parsedReviewURL.Host
				ctx.Request.Header.Del("Accept-Encoding")
				reviewProxy.ServeHTTP(ctx.Writer, ctx.Request)
			}

			v1api.Any("/reviews", reviewProxyHandler)
			v1api.Any("/reviews/*any", reviewProxyHandler)
			v1api.Any("/admin/reviews", reviewProxyHandler)
			v1api.Any("/admin/reviews/*any", reviewProxyHandler)
			// /products/{uuid}/reviews* also lands here; catalog handles the rest via NoRoute.
			isReviewProxied = true
		}
	}

	public := v1api.Group("")

	protected := v1api.Group("")
	protected.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())

	admin := v1api.Group("/admin")
	admin.Use(middleware.ApiKeyMiddleware(), middleware.AuthMiddleware())
	admin.Use(middleware.RequireRole(middleware.RoleAdmin, middleware.RoleModerator))

	for _, route := range normalRoutes {
		switch r := route.(type) {
		case *v1routes.WSRoutes, *v1routes.HealthRoutes:
			r.Register(public)
		default:
			r.Register(protected)
		}
	}

	for _, route := range adminRoutes {
		route.Register(admin)
	}

	r.NoRoute(func(ctx *gin.Context) {
		p := ctx.Request.URL.Path
		// Product review endpoints belong to the review service; everything else
		// under /products goes to catalog.
		if isReviewProxied && reviewProxyHandler != nil &&
			(strings.HasPrefix(p, "/api/v1/products/") || strings.HasPrefix(p, "/products/")) &&
			strings.Contains(p, "/reviews") {
			if !strings.HasPrefix(p, "/api/v1") {
				ctx.Request.URL.Path = "/api/v1" + p
			}
			reviewProxyHandler(ctx)
			return
		}

		if isCatalogProxied && catalogProxyHandler != nil && (strings.HasPrefix(p, "/api/v1/products") || strings.HasPrefix(p, "/products")) {
			if !strings.HasPrefix(p, "/api/v1") {
				ctx.Request.URL.Path = "/api/v1" + p
			}
			catalogProxyHandler(ctx)
			return
		}

		if !strings.HasPrefix(p, "/api/v1") && !strings.HasPrefix(p, "/internal") && !strings.HasPrefix(p, "/swagger") && !strings.HasPrefix(p, "/.well-known") {
			ctx.Request.URL.Path = "/api/v1" + p
			r.HandleContext(ctx)
			return
		}
		ctx.JSON(404, gin.H{
			"error": "Not found",
			"path":  ctx.Request.URL.Path,
		})
	})
}
