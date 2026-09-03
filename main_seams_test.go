package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CeoFred/gin-boilerplate/internal/helpers"
	"github.com/labstack/echo/v4"
)

// newSeamServer wires the routing seams the migration had to reproduce by hand and
// serves them over a real socket, so every assertion below reads bytes off the wire
// rather than a recorder's idea of what would have been written.
func newSeamServer(t *testing.T) *httptest.Server {
	t.Helper()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	table := routeTable{}
	e.HTTPErrorHandler = httpErrorHandler(&table)

	e.Pre(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Request().URL.RawPath = ""
			return next(c)
		}
	})

	appMiddleware := []echo.MiddlewareFunc{
		segmentBoundParams(),
		corsMiddleware(corsConfig{
			AllowOrigins:     []string{"http://localhost:*"},
			AllowMethods:     []string{"PUT", "PATCH", "GET", "POST", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Authorization", "Content-Type"},
			ExposeHeaders:    []string{"Content-Length"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}),
	}

	// Registered before the CORS middleware exists, exactly as the metrics endpoint is.
	e.GET("/metrics", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte("metrics"))
	})
	e.GET("/assets/*", staticHandler("/assets", "./static/public"))

	e.GET("/api/v1/ping", func(c echo.Context) error {
		return c.Blob(http.StatusOK, "text/plain; charset=utf-8", []byte("pong"))
	}, appMiddleware...)

	v1 := e.Group("/api/v1", appMiddleware...)
	v1.GET("/auth/verify/:email/:otp", func(c echo.Context) error {
		return helpers.ReturnJSON(c, "verified", c.Param("otp"), http.StatusOK)
	})
	v1.PUT("/user/", func(c echo.Context) error {
		return helpers.ReturnJSON(c, "updated", nil, http.StatusOK)
	})

	table = newRouteTable(e)

	server := httptest.NewServer(e)
	t.Cleanup(server.Close)

	return server
}

// do issues a request without following redirects, so a redirect is observable.
func do(t *testing.T, method, url string, header http.Header) (*http.Response, string) {
	t.Helper()

	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	for k, values := range header {
		for _, v := range values {
			req.Header.Add(k, v)
		}
	}

	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}

	res, err := client.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return res, string(body)
}

const notFoundBody = `{"error":"route not found","message":"Something went wrong","status":false}`

func TestUnknownRouteAnswersWithTheApplicationsOwnNotFoundBody(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/no/such/path", nil)

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
	if body != notFoundBody {
		t.Errorf("body = %q, want %q", body, notFoundBody)
	}
}

func TestWrongMethodAnswersNotFoundRatherThanMethodNotAllowed(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodPost, server.URL+"/api/v1/ping", nil)

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
	if allow := res.Header.Get("Allow"); allow != "" {
		t.Errorf("Allow = %q, want it absent", allow)
	}
	if body != notFoundBody {
		t.Errorf("body = %q, want %q", body, notFoundBody)
	}
}

func TestJSONBodiesAreCompactAndDeclareALowerCaseCharset(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/api/v1/auth/verify/a@b.com/XY", nil)

	if got, want := res.Header.Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type = %q, want %q", got, want)
	}
	if strings.HasSuffix(body, "\n") {
		t.Errorf("body %q ends with a newline", body)
	}
	if want := `{"data":"XY","message":"verified","status":true}`; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestDisallowedOriginIsRejectedWithAnEmptyForbidden(t *testing.T) {
	server := newSeamServer(t)

	for _, origin := range []string{"http://localhost:3000", "http://evil.example.com"} {
		res, body := do(t, http.MethodGet, server.URL+"/api/v1/ping", http.Header{"Origin": {origin}})

		if res.StatusCode != http.StatusForbidden {
			t.Errorf("origin %s: status = %d, want %d", origin, res.StatusCode, http.StatusForbidden)
		}
		if body != "" {
			t.Errorf("origin %s: body = %q, want empty", origin, body)
		}
	}
}

func TestPreflightForADisallowedOriginIsAlsoForbidden(t *testing.T) {
	server := newSeamServer(t)

	res, _ := do(t, http.MethodOptions, server.URL+"/api/v1/ping", http.Header{
		"Origin":                        {"http://localhost:3000"},
		"Access-Control-Request-Method": {"GET"},
	})

	if res.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusForbidden)
	}
}

func TestRoutesRegisteredBeforeCORSDoNotRunIt(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/metrics", http.Header{"Origin": {"http://localhost:3000"}})

	if res.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusOK)
	}
	if body != "metrics" {
		t.Errorf("body = %q, want %q", body, "metrics")
	}
}

func TestPathParametersDoNotSpanASlash(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/api/v1/auth/verify/a%40b.com/XY%2FZW", nil)

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
	if body != notFoundBody {
		t.Errorf("body = %q, want %q", body, notFoundBody)
	}
}

func TestAPathDifferingOnlyByItsTrailingSlashRedirects(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/api/v1/ping/", nil)
	if res.StatusCode != http.StatusMovedPermanently {
		t.Errorf("GET status = %d, want %d", res.StatusCode, http.StatusMovedPermanently)
	}
	if got := res.Header.Get("Location"); got != "/api/v1/ping" {
		t.Errorf("GET Location = %q, want %q", got, "/api/v1/ping")
	}
	if body == "" {
		t.Error("GET redirect carried no body")
	}

	res, _ = do(t, http.MethodPut, server.URL+"/api/v1/user", nil)
	if res.StatusCode != http.StatusTemporaryRedirect {
		t.Errorf("PUT status = %d, want %d", res.StatusCode, http.StatusTemporaryRedirect)
	}
	if got := res.Header.Get("Location"); got != "/api/v1/user/" {
		t.Errorf("PUT Location = %q, want %q", got, "/api/v1/user/")
	}
}

func TestAMissingStaticFileAnswersWithTheNotFoundBody(t *testing.T) {
	server := newSeamServer(t)

	res, body := do(t, http.MethodGet, server.URL+"/assets/definitely-absent.txt", nil)

	if res.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusNotFound)
	}
	if body != notFoundBody {
		t.Errorf("body = %q, want %q", body, notFoundBody)
	}
}

func TestAPanicCarryingAStringIsReportedAsText(t *testing.T) {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.GET("/boom", func(echo.Context) error { panic("kaboom") }, recoveryMiddleware())
	e.GET("/nil", func(echo.Context) error { var p *int; _ = *p; return nil }, recoveryMiddleware())

	server := httptest.NewServer(e)
	defer server.Close()

	res, body := do(t, http.MethodGet, server.URL+"/boom", nil)
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusInternalServerError)
	}
	if body != "error: kaboom" {
		t.Errorf("body = %q, want %q", body, "error: kaboom")
	}

	res, body = do(t, http.MethodGet, server.URL+"/nil", nil)
	if res.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", res.StatusCode, http.StatusInternalServerError)
	}
	if body != "" {
		t.Errorf("body = %q, want empty", body)
	}
}
