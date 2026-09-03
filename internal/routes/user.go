package routes

import (
	"github.com/labstack/echo/v4"

	deps "github.com/CeoFred/gin-boilerplate/internal/bootstrap"
	"github.com/CeoFred/gin-boilerplate/internal/handlers"
	"github.com/CeoFred/gin-boilerplate/internal/middleware"
	"github.com/CeoFred/gin-boilerplate/internal/validators"
)

func RegisterUserRoutes(router *echo.Group, d *deps.AppDependencies) {
	userRouter := router.Group("/user")

	handler := handlers.NewUserHandler(d)

	userRouter.GET("/profile", handler.UserProfile, middleware.JWTMiddleware(d.DatabaseService))
	userRouter.PUT("/", handler.UpdateUserProfile, middleware.JWTMiddleware(d.DatabaseService), validators.ValidateUpdateUserProfile)
}
