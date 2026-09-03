package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/cloudinary/cloudinary-go"
	"github.com/cloudinary/cloudinary-go/api/uploader"
	"github.com/labstack/echo/v4"

	"github.com/CeoFred/gin-boilerplate/constants"
	bootstrap "github.com/CeoFred/gin-boilerplate/internal/bootstrap"
	"github.com/CeoFred/gin-boilerplate/internal/helpers"
)

type UserHandler struct {
	deps *bootstrap.AppDependencies
}

func NewUserHandler(deps *bootstrap.AppDependencies,
) *UserHandler {
	return &UserHandler{
		deps: deps,
	}
}

type FileUploadResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type UserProfile struct {
	Email     string    `json:"email"`
	Userid    string    `json:"userid"`
	Name      string    `json:"name"`
	Country   string    `json:"country"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type UpdateUserProfileInput struct {
	PhoneNumber string `json:"phone_number" validate:"required"`
}

// UpdateUserProfile is a route handler that handles updating the user profile
//
// # This endpoint is used to update the user profile
//
// @Summary Update user profile
// @Description Updates some details about the user
// @Tags User
// @Accept json
// @Produce json
// @Param credentials body UpdateUserProfileInput true "update user profile"
// @Security BearerAuth
// @Success 200 {object} SuccessResponse
// @Failure 401 {object} ErrorResponse
// @Router /user [put]
func (u *UserHandler) UpdateUserProfile(c echo.Context) error {
	var input UpdateUserProfileInput

	validatedReqBody := c.Get("validatedRequestBody")

	if validatedReqBody == nil {
		return helpers.ReturnError(c, "Something went wrong", fmt.Errorf(helpers.INVALID_REQUEST_BODY), http.StatusBadRequest)
	}

	input, ok := validatedReqBody.(UpdateUserProfileInput)
	if !ok {
		return helpers.ReturnError(c, "Something went wrong", fmt.Errorf(helpers.REQUEST_BODY_PARSE_ERROR), http.StatusBadRequest)
	}

	claims, err := helpers.GetAuthenticatedUser(c)
	if err != nil {
		return helpers.ReturnError(c, "Something went wrong", err, http.StatusInternalServerError)
	}

	user, found, err := u.deps.UserRepo.FindByCondition("email = ?", claims.Email)
	if err != nil {
		return helpers.ReturnError(c, "Something went wrong", err, http.StatusInternalServerError)
	}

	if !found {
		return helpers.ReturnError(c, "Something went wrong", fmt.Errorf("user not found"), http.StatusNotFound)
	}

	user.PhoneNumber = input.PhoneNumber

	_, err = u.deps.UserRepo.Save(user)
	if err != nil {
		return helpers.ReturnError(c, "Something went wrong", err, http.StatusInternalServerError)
	}

	return helpers.ReturnJSON(c, "Profile updated successfully", user, http.StatusOK)
}

// UserProfile is a route handler that retrieves the user profile of the authenticated user.
//
// This endpoint is used to get the profile information of the authenticated user based on the JWT claims.
//
// @Summary Get user profile
// @Description Retrieves the profile information of the authenticated user
// @Tags User
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.User
// @Failure 401 {object} ErrorResponse
// @Router /user/profile [get]
func (u *UserHandler) UserProfile(c echo.Context) error {

	claimsRaw := c.Get("claims")

	if claimsRaw == nil {
		return writeJSON(c, http.StatusUnauthorized, map[string]interface{}{"error": "Unauthorized"})
	}

	authClaims, ok := claimsRaw.(*helpers.AuthTokenJwtClaim)
	if !ok {
		return writeJSON(c, http.StatusUnauthorized, map[string]interface{}{"error": "Unauthorized"})
	}

	user, f, err := u.deps.UserRepo.FindByCondition("user_id = ?", authClaims.UserId)

	if err != nil {
		return helpers.ReturnError(c, "Something went wrong", err, http.StatusInternalServerError)
	}

	if !f {
		return helpers.ReturnError(c, "Something went wrong", fmt.Errorf("account not found"), http.StatusNotFound)
	}

	return helpers.ReturnJSON(c, "Profile retrieved", user, http.StatusOK)

}

func uploadFile(file *multipart.FileHeader) (resp *uploader.UploadResult, err error) {

	env_ := constants.New()
	// Open the uploaded file
	fileOpened, err := file.Open()
	if err != nil {
		// Handle error
		return nil, err
	}

	defer fileOpened.Close()
	url := fmt.Sprintf("cloudinary://%s:%s@%s", env_.CloudinaryAPIKey, env_.CloudinaryApiSecret, env_.CloudinaryName)

	cld, err := cloudinary.NewFromURL(url)

	if err != nil {
		return nil, err
	}
	var ctx = context.Background()
	// Upload the image to Cloudinary
	resp, err = cld.Upload.Upload(ctx, fileOpened, uploader.UploadParams{PublicID: file.Filename,
		Folder: "BonpayFiance",
	})

	if err != nil {
		return nil, err
	}

	return resp, nil
}

// NotFound returns custom 404 page
func NotFound(c echo.Context) error {
	c.Response().Status = 404
	return c.File("./static/private/404.html")
}

// writeJSON mirrors what gin wrote for the two handlers that build a response envelope
// inline: a compact body typed "application/json; charset=utf-8". echo.Context.JSON
// would append a newline and spell the charset "UTF-8".
func writeJSON(c echo.Context, statusCode int, payload interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return c.Blob(statusCode, "application/json; charset=utf-8", body)
}
