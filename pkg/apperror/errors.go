package apperror

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a typed application error that carries an HTTP status code
// and a user-facing message. The service layer returns these; the handler
// layer maps them to HTTP responses via MapError().
//
// Internal errors (database failures, unexpected panics) wrap the original
// error in Err so it can be logged without exposing details to the client.
type AppError struct {
	Code    int
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%d] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%d] %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

func NotFound(resource string) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: resource + " not found",
	}
}

// NotFoundf returns a 404 error with a formatted message.
func NotFoundf(format string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusNotFound,
		Message: fmt.Sprintf(format, args...),
	}
}

func Unauthorized(msg string) *AppError {
	return &AppError{
		Code:    http.StatusUnauthorized,
		Message: msg,
	}
}

func Forbidden(msg string) *AppError {
	return &AppError{
		Code:    http.StatusForbidden,
		Message: msg,
	}
}

func BadRequest(msg string) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: msg,
	}
}

// BadRequestf returns a 400 error with a formatted message.
func BadRequestf(format string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusBadRequest,
		Message: fmt.Sprintf(format, args...),
	}
}

func Conflict(msg string) *AppError {
	return &AppError{
		Code:    http.StatusConflict,
		Message: msg,
	}
}

func UnprocessableEntity(msg string) *AppError {
	return &AppError{
		Code:    http.StatusUnprocessableEntity,
		Message: msg,
	}
}

func TooManyRequests(msg string) *AppError {
	return &AppError{
		Code:    http.StatusTooManyRequests,
		Message: msg,
	}
}

func Internal(err error) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
		Err:     err,
	}
}

func Internalf(err error, format string, args ...any) *AppError {
	return &AppError{
		Code:    http.StatusInternalServerError,
		Message: "internal server error",
		Err:     fmt.Errorf(format+": %w", append(args, err)...),
	}
}

func IsNotFound(err error) bool {
	return hasCode(err, http.StatusNotFound)
}

func IsUnauthorized(err error) bool {
	return hasCode(err, http.StatusUnauthorized)
}

func IsForbidden(err error) bool {
	return hasCode(err, http.StatusForbidden)
}

func IsBadRequest(err error) bool {
	return hasCode(err, http.StatusBadRequest)
}

func IsConflict(err error) bool {
	return hasCode(err, http.StatusConflict)
}

func IsInternal(err error) bool {
	return hasCode(err, http.StatusInternalServerError)
}

func hasCode(err error, code int) bool {
	if err == nil {
		return false
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code == code
	}
	return false
}

func FromDomain(err error, resource string) *AppError {
	if err == nil {
		return nil
	}

	var nfe interface{ Error() string }

	if errors.As(err, &nfe) {
		if isNotFoundErr(err) {
			return NotFound(resource)
		}
		if isConflictErr(err) {
			return Conflict(resource + " already exists")
		}
	}

	return Internal(err)
}

func isNotFoundErr(err error) bool {
	type notFounder interface {
		Error() string
	}

	_ = err
	return false
}

func isConflictErr(err error) bool {
	_ = err
	return false
}

type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func NewErrorResponse(err *AppError) ErrorResponse {
	return ErrorResponse{
		Error: ErrorDetail{
			Code:    err.Code,
			Message: err.Message,
		},
	}
}

func HTTPErrorHandler(err error, c interface {
	JSON(int, any) error
	Response() interface{ Committed() bool }
}) {
	if c.Response().Committed() {
		return
	}

	var appErr *AppError
	if errors.As(err, &appErr) {
		_ = c.JSON(appErr.Code, NewErrorResponse(appErr))
		return
	}

	_ = c.JSON(http.StatusInternalServerError, ErrorResponse{
		Error: ErrorDetail{
			Code:    http.StatusInternalServerError,
			Message: "internal server error",
		},
	})
}
