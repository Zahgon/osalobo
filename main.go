package main

import (
	"context"
	"log"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/CeoFred/gin-boilerplate/constants"
	"github.com/CeoFred/gin-boilerplate/database"
	"github.com/CeoFred/gin-boilerplate/internal/bootstrap"
	"github.com/CeoFred/gin-boilerplate/internal/handlers"
	"github.com/CeoFred/gin-boilerplate/internal/helpers"
	"github.com/CeoFred/gin-boilerplate/internal/otp"
	"github.com/CeoFred/gin-boilerplate/internal/routes"
	"github.com/CeoFred/gin-boilerplate/internal/service/streaming"

	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	_ "golang.org/x/text"

	docs "github.com/CeoFred/gin-boilerplate/docs"
	apitoolkit "github.com/apitoolkit/apitoolkit-go"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	echoSwagger "github.com/swaggo/echo-swagger"
)

// Define metrics
var (
	httpRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	httpRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"method", "endpoint"},
	)

	activeConnections = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_active_connections",
			Help: "Number of active HTTP connections",
		},
	)
)

func prometheusMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Increment active connections
			activeConnections.Inc()

			// Process request
			// The response is written here rather than after the chain unwinds so the
			// status observed below is the one the client received.
			if err := next(c); err != nil {
				c.Error(err)
			}

			// Decrement active connections
			activeConnections.Dec()

			duration := time.Since(start).Seconds()

			// Skip metrics endpoint itself
			if c.Request().URL.Path != "/metrics" {
				status := strconv.Itoa(c.Response().Status)
				httpRequestsTotal.WithLabelValues(c.Request().Method, c.Request().URL.Path, status).Inc()
				httpRequestDuration.WithLabelValues(c.Request().Method, c.Request().URL.Path).Observe(duration)
			}

			return nil
		}
	}
}

// recoveryMiddleware turns a panic into a bare 500, and a panic carrying a string into
// "error: <value>" as text. It never rewrites an already committed response.
func recoveryMiddleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("panic recovered: %v", recovered)

					if c.Response().Committed {
						err = nil
						return
					}

					if message, ok := recovered.(string); ok {
						err = c.Blob(http.StatusInternalServerError, "text/plain; charset=utf-8",
							[]byte(fmt.Sprintf("error: %s", message)))
						return
					}

					err = c.NoContent(http.StatusInternalServerError)
				}
			}()

			return next(c)
		}
	}
}

// corsConfig mirrors the policy the application declares. An origin is compared
// literally, so a value such as "http://localhost:*" matches nothing.
type corsConfig struct {
	AllowOrigins     []string
	AllowMethods     []string
	AllowHeaders     []string
	ExposeHeaders    []string
	AllowCredentials bool
	MaxAge           time.Duration
}

func (cfg corsConfig) allows(origin string) bool {
	for _, allowed := range cfg.AllowOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

// corsMiddleware rejects a cross-origin request whose Origin is not allowed with a bare
// 403, and answers an allowed preflight with 204. echo's own CORS middleware reads "*"
// inside an origin as a wildcard pattern and lets a disallowed origin through without
// the CORS headers, so neither half of that contract survives it.
func corsMiddleware(cfg corsConfig) echo.MiddlewareFunc {
	allowMethods := strings.Join(cfg.AllowMethods, ",")
	allowHeaders := strings.Join(cfg.AllowHeaders, ",")
	exposeHeaders := strings.Join(cfg.ExposeHeaders, ",")
	maxAge := strconv.Itoa(int(cfg.MaxAge / time.Second))

	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()

			origin := req.Header.Get(echo.HeaderOrigin)
			if origin == "" {
				// request is not a CORS request
				return next(c)
			}

			host := req.Host
			if origin == "http://"+host || origin == "https://"+host {
				// request is not a CORS request but have origin header.
				return next(c)
			}

			if !cfg.allows(origin) {
				return c.NoContent(http.StatusForbidden)
			}

			res.Header().Add(echo.HeaderVary, echo.HeaderOrigin)
			res.Header().Set(echo.HeaderAccessControlAllowOrigin, origin)
			if cfg.AllowCredentials {
				res.Header().Set(echo.HeaderAccessControlAllowCredentials, "true")
			}

			if req.Method != http.MethodOptions {
				if exposeHeaders != "" {
					res.Header().Set(echo.HeaderAccessControlExposeHeaders, exposeHeaders)
				}
				return next(c)
			}

			res.Header().Add(echo.HeaderVary, echo.HeaderAccessControlRequestMethod)
			res.Header().Add(echo.HeaderVary, echo.HeaderAccessControlRequestHeaders)
			res.Header().Set(echo.HeaderAccessControlAllowMethods, allowMethods)
			if allowHeaders != "" {
				res.Header().Set(echo.HeaderAccessControlAllowHeaders, allowHeaders)
			}
			if cfg.MaxAge > 0 {
				res.Header().Set(echo.HeaderAccessControlMaxAge, maxAge)
			}

			return c.NoContent(http.StatusNoContent)
		}
	}
}

// segmentBoundParams rejects a request whose path parameter spans a "/". A trailing
// parameter with no children is otherwise matched greedily to the end of the path, so a
// route declaring two parameters would answer a path carrying three segments.
func segmentBoundParams() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			values := c.ParamValues()
			for i, name := range c.ParamNames() {
				if name == "*" || i >= len(values) {
					continue
				}
				if strings.Contains(values[i], "/") {
					return echo.ErrNotFound
				}
			}
			return next(c)
		}
	}
}

// onlyFilesFS serves files but never a directory listing.
type onlyFilesFS struct {
	fs http.FileSystem
}

func (o onlyFilesFS) Open(name string) (http.File, error) {
	f, err := o.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return neuteredReaddirFile{f}, nil
}

type neuteredReaddirFile struct {
	http.File
}

func (f neuteredReaddirFile) Readdir(count int) ([]os.FileInfo, error) {
	return nil, nil
}

// staticHandler serves root under urlPrefix. A miss is reported as a routing failure so
// it is answered by the same not-found body as any other unknown path.
func staticHandler(urlPrefix, root string) echo.HandlerFunc {
	fs := onlyFilesFS{http.Dir(root)}
	fileServer := http.StripPrefix(urlPrefix, http.FileServer(fs))

	return func(c echo.Context) error {
		file := c.Param("*")

		// Check if file exists and/or if we have permission to access it
		f, err := fs.Open("/" + file)
		if err != nil {
			return echo.ErrNotFound
		}
		f.Close()

		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}

// routeTable answers whether a concrete path is served, so a request that differs from a
// real route only by its trailing slash can be redirected onto it.
type routeTable map[string][]string

func newRouteTable(e *echo.Echo) routeTable {
	table := routeTable{}
	for _, r := range e.Routes() {
		table[r.Method] = append(table[r.Method], r.Path)
	}
	return table
}

func (t routeTable) match(method, path string) bool {
	for _, pattern := range t[method] {
		if matchRoutePattern(pattern, path) {
			return true
		}
	}
	return false
}

func matchRoutePattern(pattern, path string) bool {
	patternParts := strings.Split(pattern, "/")
	pathParts := strings.Split(path, "/")

	for i, part := range patternParts {
		if part == "*" {
			return true
		}
		if i >= len(pathParts) {
			return false
		}
		if strings.HasPrefix(part, ":") {
			if pathParts[i] == "" {
				return false
			}
			continue
		}
		if part != pathParts[i] {
			return false
		}
	}

	return len(patternParts) == len(pathParts)
}

func redirectTrailingSlash(table routeTable, c echo.Context) bool {
	req := c.Request()
	path := req.URL.Path

	if table.match(req.Method, path) {
		return false
	}

	var alternate string
	if len(path) > 1 && strings.HasSuffix(path, "/") {
		alternate = strings.TrimSuffix(path, "/")
	} else {
		alternate = path + "/"
	}

	if alternate == "" || !table.match(req.Method, alternate) {
		return false
	}

	code := http.StatusMovedPermanently
	if req.Method != http.MethodGet {
		code = http.StatusTemporaryRedirect
	}

	target := *req.URL
	target.Path = alternate
	http.Redirect(c.Response(), req, target.RequestURI(), code)

	return true
}

// httpErrorHandler answers an unrouted request, including one that only differs from a
// route by its method, with the application's own not-found body.
func httpErrorHandler(table *routeTable) echo.HTTPErrorHandler {
	return func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		code := http.StatusInternalServerError
		if httpError, ok := err.(*echo.HTTPError); ok {
			code = httpError.Code
		}

		if code == http.StatusNotFound || code == http.StatusMethodNotAllowed {
			if redirectTrailingSlash(*table, c) {
				return
			}
			helpers.ReturnError(c, "Something went wrong", fmt.Errorf("route not found"), http.StatusNotFound)
			return
		}

		c.NoContent(code)
	}
}

// @title Gin Boilerplare
// @version 1.0
// @description Swagger API documentation for Gin Boilerplare API
// @termsOfService http://swagger.io/terms/
// @contact.name Johnson Awah Alfred
// @contact.email johnsonmessilo19@gmail.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host https://example.com
// @BasePath /api/v1
func main() {

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Debug = os.Getenv("ECHO_DEBUG") != "false"

	table := routeTable{}
	e.HTTPErrorHandler = httpErrorHandler(&table)

	// Match routes against the decoded path, so "%2F" separates segments instead of
	// staying inside a single path parameter.
	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Request().URL.RawPath = ""
			return next(c)
		}
	})

	// The chain a route runs is fixed when the route is registered, so each group below
	// carries only the middleware that had been declared by that point.
	baseMiddleware := []echo.MiddlewareFunc{middleware.Logger(), recoveryMiddleware()}

	docs.SwaggerInfo.BasePath = "/api/v1"

	constant := constants.New()
	_ = otp.NewOTPManager()

	ctx := context.Background()

	v := constants.New()

	apitoolkitClient, err := apitoolkit.NewClient(ctx, apitoolkit.Config{APIKey: v.APIToolkitKey})

	if err != nil {
		log.Println(err)
	} else {
		baseMiddleware = append(baseMiddleware, apitoolkitClient.EchoMiddleware)
	}

	flag.Parse()

	staticMiddleware := append([]echo.MiddlewareFunc{}, baseMiddleware...)
	e.GET("/assets/*", staticHandler("/assets", "./static/public"), staticMiddleware...)
	e.HEAD("/assets/*", staticHandler("/assets", "./static/public"), staticMiddleware...)
	e.GET("/templates/*", staticHandler("/templates", "./templates"), staticMiddleware...)
	e.HEAD("/templates/*", staticHandler("/templates", "./templates"), staticMiddleware...)

	metricsMiddleware := append([]echo.MiddlewareFunc{}, baseMiddleware...)
	metricsMiddleware = append(metricsMiddleware, recoveryMiddleware(), middleware.Logger(), prometheusMiddleware())

	e.GET("/metrics", echo.WrapHandler(promhttp.Handler()), metricsMiddleware...)

	appMiddleware := append([]echo.MiddlewareFunc{}, metricsMiddleware...)
	appMiddleware = append(appMiddleware, segmentBoundParams())
	appMiddleware = append(appMiddleware, middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "${remote_ip} - [${time_custom}] \"${method} ${path} ${protocol} ${status} ${latency_human} " +
			"\"${user_agent}\" ${error}\"\n",
		CustomTimeFormat: time.RFC1123,
	}))

	appMiddleware = append(appMiddleware, corsMiddleware(corsConfig{
		AllowOrigins:     []string{"http://localhost:*"},
		AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	appMiddleware = append(appMiddleware, apitoolkitClient.EchoMiddleware)

	dbConfig := database.Config{
		Host:     v.DbHost,
		Port:     v.DbPort,
		Password: v.DbPassword,
		User:     v.DbUser,
		DBName:   v.DbName,
	}

	database.Connect(&dbConfig)

	connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s", v.DbUser, v.DbPassword, v.DbHost, v.DbPort, v.DbName, v.SSLMode)
	database.RunManualMigration(connStr)

	// use echoSwagger middleware to serve the API docs
	docs.SwaggerInfo.BasePath = "/api/v1"
	e.GET("/swagger/*", echoSwagger.EchoWrapHandler(echoSwagger.URL("/swagger/doc.json")), appMiddleware...)

	e.GET("/api/v1/ping", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte("pong"))
	}, appMiddleware...)

	v1 := e.Group("/api/v1", appMiddleware...)

	keepRunning := true

	sqlDB, err := database.DB.DB()
	if err != nil {
		log.Fatal("Error getting underlying SQL DB:", err)
	}

	provider, err := streaming.NewProducer(&streaming.Config{
		Verbose:   false,
		Producers: 3,
		Topic:     []string{"signup"},
		Version:   "3.8.0",
		Brokers:   "127.0.0.1:9092",
	})

	if err != nil {
		log.Fatal(err)
	}

	dependencies := bootstrap.InitializeDependencies(database.DB)
	dependencies.EventProducer = provider

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	consumerClient, err := streaming.NewConsumer(&streaming.Config{
		Verbose:  true,
		Version:  "3.8.0",
		Brokers:  "127.0.0.1:9092",
		Assignor: "roundrobin",
		Oldest:   true,
		Group:    "gin",
		Ctx:      ctx,
	})

	if err != nil {
		log.Fatal(err)
	}

	eventHandler := handlers.EventHandler{
		Deps: dependencies,
	}

	if err := consumerClient.Consume("signup", eventHandler.ProcessSignup); err != nil {
		log.Fatal(err)
	}

	routes.Routes(v1, dependencies)

	// Every request the engine does not route is answered by one no-route handler that
	// carries the whole root middleware chain. Echo has no equivalent, so the same
	// fallback is registered explicitly: without it the router answers a wrong method
	// with 405 + `Allow`, and an OPTIONS with its own 204, before httpErrorHandler can
	// run. A `*` node ends the router's backtracking, so every wildcard family needs its
	// own fallback as well as the root one. /api/v1/* is left to the group, which already
	// registers the same fallback for itself.
	for _, path := range []string{"", "/*", "/assets/*", "/templates/*", "/swagger/*"} {
		e.RouteNotFound(path, echo.NotFoundHandler, appMiddleware...)
	}

	table = newRouteTable(e)

	port := os.Getenv("PORT")

	if port == "" {
		port = constant.Port
	}

	go log.Fatal(e.Start(":" + port))

	sigterm := make(chan os.Signal, 1)
	sigusr1 := make(chan os.Signal, 1)

	signal.Notify(sigterm, syscall.SIGINT, syscall.SIGTERM)
	signal.Notify(sigusr1, syscall.SIGUSR1)

	for keepRunning {
		select {
		case <-ctx.Done():
			log.Println("terminating: context cancelled")
			keepRunning = false
		case <-sigterm:
			log.Println("terminating: via signal")
			keepRunning = false
		case <-sigusr1:
			consumerClient.ToggleConsumptionFlow()
		}
	}

	provider.Clear()
	cancel()

	if err := sqlDB.Close(); err != nil {
		log.Fatal("Error closing database connection:", err)
	}

	log.Println("Database connection closed")
}
