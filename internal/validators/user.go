package validators

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/CeoFred/gin-boilerplate/internal/handlers"
	"github.com/CeoFred/gin-boilerplate/internal/helpers"
	"github.com/CeoFred/gin-boilerplate/validator"

	govalidator "github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
)

func ValidateRegisterUserSchema(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.InputCreateUser
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateAccountResetScheme(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body helpers.AccountReset
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateOTPVerifySchema(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body helpers.OtpVerify
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateLoginUser(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.AuthenticateUser
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateUpdateUserProfile(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.UpdateUserProfileInput
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateResetUserSchema(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.ForgotPasswordInput
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateResetPasswordSchema(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.ResetPasswordInput
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

func ValidateResetOTPVerifySchema(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		var body handlers.OtpVerifyInput
		if ok, err := bindAndValidate(c, &body); !ok {
			return err
		}
		c.Set("validatedRequestBody", body)
		return next(c)
	}
}

// bindingValidate enforces the `binding` struct tags on request bodies. echo.Context.Bind
// neither reads that tag nor accepts a request without a JSON content type, so the decode
// and the tag check are done directly here instead.
var bindingValidate = func() *govalidator.Validate {
	v := govalidator.New()
	v.SetTagName("binding")
	return v
}()

func bindJSON(c echo.Context, body interface{}) error {
	req := c.Request()
	if req == nil || req.Body == nil {
		return errors.New("invalid request")
	}

	if err := json.NewDecoder(req.Body).Decode(body); err != nil {
		return err
	}

	return bindingValidate.Struct(body)
}

// bindAndValidate reports whether the body was accepted. On rejection it has already
// written the 400 response, so the caller must stop the chain rather than call the handler.
func bindAndValidate(c echo.Context, body interface{}) (bool, error) {
	if err := bindJSON(c, body); err != nil {
		return false, helpers.ReturnError(c, "Something went wrong", err, http.StatusBadRequest)
	}

	if err := validator.Validate(body); err != nil {
		return false, helpers.ReturnError(c, "Something went wrong", err, http.StatusBadRequest)
	}

	return true, nil
}
