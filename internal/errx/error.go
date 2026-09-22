package errx

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Error is the single application error type. Every error that can reach a
// transport boundary must be an *Error so the middleware can map it to the
// correct HTTP status without string-matching.
type Error struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Type       Type                   `json:"type"`
	HTTPStatus int                    `json:"http_status"`
	Details    map[string]interface{} `json:"details,omitempty"`
	Err        error                  `json:"-"`
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func (e *Error) WithDetail(key string, value interface{}) *Error {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// New creates a typed error with the correct HTTP status derived from the type.
func New(message string, errType Type) *Error {
	return &Error{
		Code:       string(errType),
		Message:    message,
		Type:       errType,
		HTTPStatus: typeToHTTPStatus(errType),
		Details:    make(map[string]interface{}),
	}
}

// Wrap wraps an existing error with context. If the underlying error is already
// an *Error, its code/type/status are preserved so re-wrapping never downgrades
// a 404 into a 500.
func Wrap(err error, message string, errType Type) *Error {
	if err == nil {
		return nil
	}
	var existing *Error
	if errors.As(err, &existing) {
		return &Error{
			Code:       existing.Code,
			Message:    message,
			Type:       existing.Type,
			HTTPStatus: existing.HTTPStatus,
			Details:    existing.Details,
			Err:        err,
		}
	}
	return &Error{
		Code:       string(errType),
		Message:    message,
		Type:       errType,
		HTTPStatus: typeToHTTPStatus(errType),
		Details:    make(map[string]interface{}),
		Err:        err,
	}
}

// Wrapf is like Wrap but with format string.
func Wrapf(err error, errType Type, format string, args ...interface{}) *Error {
	return Wrap(err, fmt.Sprintf(format, args...), errType)
}

// Is delegates to errors.Is.
func Is(err, target error) bool { return errors.Is(err, target) }

// As delegates to errors.As.
func As(err error, target interface{}) bool { return errors.As(err, target) }

func typeToHTTPStatus(t Type) int {
	switch t {
	case TypeValidation:
		return 400
	case TypeAuthorization:
		return 401
	case TypeForbidden:
		return 403
	case TypeNotFound:
		return 404
	case TypeConflict:
		return 409
	case TypeBusiness:
		return 422
	case TypeRateLimited:
		return 429
	case TypeExternal:
		return 502
	case TypeInternal:
		return 500
	default:
		return 500
	}
}

// HTTPErrorResponse is the JSON body sent to clients on error.
type HTTPErrorResponse struct {
	Code       string                 `json:"code"`
	Message    string                 `json:"message"`
	Type       string                 `json:"type"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    `json:"status_code"`
}

// ToHTTPResponse converts the error into a JSON-safe response struct.
func (e *Error) ToHTTPResponse() HTTPErrorResponse {
	return HTTPErrorResponse{
		Code:       e.Code,
		Message:    e.Message,
		Type:       string(e.Type),
		Details:    e.Details,
		StatusCode: e.HTTPStatus,
	}
}

// MarshalJSON implements json.Marshaler for the Error type.
func (e *Error) MarshalJSON() ([]byte, error) {
	return json.Marshal(e.ToHTTPResponse())
}
