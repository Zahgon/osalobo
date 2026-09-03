package routes

import (
	"github.com/labstack/echo/v4"

	deps "github.com/CeoFred/gin-boilerplate/internal/bootstrap"
	"github.com/CeoFred/gin-boilerplate/internal/handlers"
	"github.com/CeoFred/gin-boilerplate/internal/validators"
)

func RegisterAuthRoutes(router *echo.Group, d *deps.AppDependencies) {
	handler := handlers.NewAuthHandler(d)

	authRouter := router.Group("/auth")

	authRouter.POST("/register", handler.Register, validators.ValidateRegisterUserSchema)
	authRouter.POST("/login", handler.Authenticate, validators.ValidateLoginUser)
	authRouter.GET("/verify/:email/:otp", handler.VerifyEmail)
	authRouter.POST("/forgot-password/verify", handler.VerifyResetOTP, validators.ValidateResetOTPVerifySchema)
	authRouter.POST("/forgot-password", handler.ForgotPassword, validators.ValidateResetUserSchema)
	authRouter.POST("/reset-password/confirm/:reset-token", handler.ResetPassword, validators.ValidateResetPasswordSchema)
}
