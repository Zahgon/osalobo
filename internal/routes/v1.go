package routes

import (
	"github.com/labstack/echo/v4"

	"github.com/CeoFred/gin-boilerplate/internal/bootstrap"
)

func Routes(r *echo.Group, d *bootstrap.AppDependencies) {
	RegisterUserRoutes(r, d)
	RegisterAuthRoutes(r, d)
}
