package validators

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CeoFred/gin-boilerplate/internal/handlers"
	"github.com/labstack/echo/v4"
)

// newBindServer serves one route guarded by the register validator, over a real socket,
// and reports the body the handler saw when the request was accepted.
func newBindServer(t *testing.T) *httptest.Server {
	t.Helper()

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	e.POST("/register", func(c echo.Context) error {
		body, ok := c.Get("validatedRequestBody").(handlers.InputCreateUser)
		if !ok {
			return c.String(http.StatusInternalServerError, "no validated body")
		}
		return c.String(http.StatusOK, body.Email)
	}, ValidateRegisterUserSchema)

	server := httptest.NewServer(e)
	t.Cleanup(server.Close)

	return server
}

func post(t *testing.T, url, contentType, body string) (int, string) {
	t.Helper()

	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	if contentType != "" {
		req.Header.Set(echo.HeaderContentType, contentType)
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post: %v", err)
	}
	defer res.Body.Close()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}

	return res.StatusCode, string(payload)
}

const validRegistration = `{"email":"a@b.com","password":"p","first_name":"A","last_name":"B"}`

func TestABodyMissingRequiredFieldsIsRejectedWithTheTagErrorText(t *testing.T) {
	server := newBindServer(t)

	status, body := post(t, server.URL+"/register", echo.MIMEApplicationJSON, `{}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	for _, field := range []string{"Email", "Password", "FirstName", "LastName"} {
		want := "Key: 'InputCreateUser." + field + "' Error:Field validation for '" + field + "' failed on the 'required' tag"
		if !strings.Contains(body, want) {
			t.Errorf("body %q is missing %q", body, want)
		}
	}
}

func TestAMalformedEmailIsRejected(t *testing.T) {
	server := newBindServer(t)

	status, body := post(t, server.URL+"/register", echo.MIMEApplicationJSON,
		`{"email":"not-an-email","password":"p","first_name":"A","last_name":"B"}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if want := "failed on the 'email' tag"; !strings.Contains(body, want) {
		t.Errorf("body %q is missing %q", body, want)
	}
}

func TestAJSONBodyIsAcceptedWhateverTheContentTypeClaims(t *testing.T) {
	server := newBindServer(t)

	for _, contentType := range []string{"", "text/plain", echo.MIMEApplicationJSON} {
		status, body := post(t, server.URL+"/register", contentType, validRegistration)

		if status != http.StatusOK {
			t.Errorf("content type %q: status = %d, want %d (body %q)", contentType, status, http.StatusOK, body)
		}
		if body != "a@b.com" {
			t.Errorf("content type %q: body = %q, want %q", contentType, body, "a@b.com")
		}
	}
}

func TestAnUnparseableBodyIsRejected(t *testing.T) {
	server := newBindServer(t)

	for _, payload := range []string{"", "{not json", "[]"} {
		status, _ := post(t, server.URL+"/register", echo.MIMEApplicationJSON, payload)

		if status != http.StatusBadRequest {
			t.Errorf("payload %q: status = %d, want %d", payload, status, http.StatusBadRequest)
		}
	}
}

func TestARejectedBodyStopsTheChain(t *testing.T) {
	server := newBindServer(t)

	status, body := post(t, server.URL+"/register", echo.MIMEApplicationJSON, `{}`)

	if status != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", status, http.StatusBadRequest)
	}
	if strings.Contains(body, "no validated body") {
		t.Error("the handler ran even though the body was rejected")
	}
}
