package services

import "fmt"

// APIError carries an HTTP status and message back to the JSON layer.
type APIError struct {
	Status int
	Msg    string
}

func (e *APIError) Error() string { return e.Msg }

func BadRequest(format string, a ...any) error {
	return &APIError{400, fmt.Sprintf(format, a...)}
}
func Unauthorized(format string, a ...any) error {
	return &APIError{401, fmt.Sprintf(format, a...)}
}
func NotFound(format string, a ...any) error {
	return &APIError{404, fmt.Sprintf(format, a...)}
}
func Conflict(format string, a ...any) error {
	return &APIError{409, fmt.Sprintf(format, a...)}
}
func ServerError(format string, a ...any) error {
	return &APIError{500, fmt.Sprintf(format, a...)}
}