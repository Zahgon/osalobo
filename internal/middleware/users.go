package middleware

import (
	"net/http"

	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"

	"github.com/CeoFred/gin-boilerplate/constants"
	"github.com/CeoFred/gin-boilerplate/internal/helpers"
	"github.com/CeoFred/gin-boilerplate/internal/models"
	"github.com/CeoFred/gin-boilerplate/internal/repository"

	"strings"
)

type AppError struct {
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}
func NewError(message string) *AppError {
	return &AppError{
		Message: message,
	}
}

var (
	constant = constants.New()
)

func OnlyAdmin(db *gorm.DB, u *repository.UserRepository) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			claims := c.Get("claims")
			claimsData, ok := claims.(*helpers.AuthTokenJwtClaim)
			if !ok {
				return helpers.ReturnJSON(c, "Something went wrong", nil, http.StatusUnauthorized)
			}

			user, _, err := u.FindByCondition("id", claimsData.UserId)
			if err != nil {
				return helpers.ReturnError(c, "Something went wrong", err, http.StatusUnauthorized)
			}
			if user.Role != models.AdminRole {
				return helpers.ReturnJSON(c, "Unauthorized access to resource", nil, http.StatusUnauthorized)
			}

			c.Set("role", user.Role)
			return next(c)
		}
	}
}

func JWTMiddleware(db *gorm.DB) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Get the JWT token from the Authorization header
			authHeader := c.Request().Header.Get("Authorization")

			apiQuery := c.QueryParam("access_token")

			if apiQuery != "" {
				authHeader = apiQuery
			}

			if authHeader == "" {
				return helpers.ReturnJSON(c, "Missing Authorization Header or Access Token", nil, http.StatusUnauthorized)
			}

			// Extract the token from the "Bearer <jwt>" format
			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == "" {
				return helpers.ReturnJSON(c, "Invalid Authorization Header or Access Token", nil, http.StatusUnauthorized)
			}

			// Parse and validate the JWT token
			token, err := jwt.ParseWithClaims(tokenString, &helpers.AuthTokenJwtClaim{}, func(token *jwt.Token) (interface{}, error) {
				// Provide the same JWT secret key used for signing the tokens
				return []byte(constant.JWTSecretKey), nil
			})
			if err != nil || !token.Valid {
				return helpers.ReturnError(c, "Expired Authorization or Access Token", err, http.StatusUnauthorized)
			}

			// Extract the claims from the token
			claims, ok := token.Claims.(*helpers.AuthTokenJwtClaim)
			if !ok {
				return helpers.ReturnJSON(c, "Invalid claims", nil, http.StatusUnauthorized)
			}

			// Attach the claims to the request context for further use
			c.Set("claims", claims)

			_, found, err := repository.NewUserRepository(db).FindByCondition("email", claims.Email)

			if err != nil {
				return helpers.ReturnError(c, "Something went wrong", err, http.StatusUnauthorized)
			}

			if !found {
				return helpers.ReturnJSON(c, "Unauthorized access to resource", nil, http.StatusUnauthorized)
			}

			// Proceed to the next middleware or route handler
			return next(c)
		}
	}
}
