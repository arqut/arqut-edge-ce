package api

import (
	"github.com/gofiber/fiber/v2"
)

// SuccessResp sends a successful API response
func SuccessResp(c *fiber.Ctx, data interface{}, meta ...ApiResponseMeta) error {
	resp := ApiResponse{
		Success: true,
		Data:    data,
	}
	if len(meta) > 0 {
		resp.Meta = &meta[0]
	}
	return c.Status(fiber.StatusOK).JSON(&resp)
}

// ErrorResp sends an error API response
func ErrorResp(c *fiber.Ctx, err ApiError, meta ...ApiResponseMeta) error {
	resp := ApiResponse{
		Success: false,
		Error:   &err,
	}
	if len(meta) > 0 {
		resp.Meta = &meta[0]
	}
	code := fiber.StatusBadRequest
	if err.Code != 0 {
		code = err.Code
	}
	return c.Status(code).JSON(&resp)
}

// ErrorCodeResp sends an error response with a specific status code
func ErrorCodeResp(c *fiber.Ctx, code int, message ...string) error {
	msg := "API Error"
	if len(message) > 0 {
		msg = message[0]
	}
	return ErrorResp(c, ApiError{
		Code:    code,
		Message: msg,
	})
}

// ErrorNotFoundResp sends a 404 Not Found error response
func ErrorNotFoundResp(c *fiber.Ctx, message ...string) error {
	return ErrorCodeResp(c, fiber.StatusNotFound, message...)
}

// ErrorUnauthorizedResp sends a 401 Unauthorized error response
func ErrorUnauthorizedResp(c *fiber.Ctx, message ...string) error {
	return ErrorCodeResp(c, fiber.StatusUnauthorized, message...)
}

// ErrorBadRequestResp sends a 400 Bad Request error response
func ErrorBadRequestResp(c *fiber.Ctx, message ...string) error {
	return ErrorCodeResp(c, fiber.StatusBadRequest, message...)
}

// ErrorInternalServerErrorResp sends a 500 Internal Server Error response
func ErrorInternalServerErrorResp(c *fiber.Ctx, message ...string) error {
	return ErrorCodeResp(c, fiber.StatusInternalServerError, message...)
}
