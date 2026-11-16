// Package errors defines the API error envelope {code, message} and the
// status-to-code-name mapping mirroring the Rust backend.
package errors

type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return e.Message
}

func statusName(code int) string {
	switch code {
	case 400:
		return "invalid_argument"
	case 401:
		return "unauthenticated"
	case 403:
		return "permission_denied"
	case 404:
		return "not_found"
	case 409:
		return "already_exists"
	case 429:
		return "resource_exhausted"
	case 502:
		return "upstream_unavailable"
	default:
		return "internal"
	}
}

func BadRequest(msg string) *Error      { return &Error{400, "invalid_argument", msg} }
func Unauthorized(msg string) *Error    { return &Error{401, "unauthenticated", msg} }
func Forbidden(msg string) *Error       { return &Error{403, "permission_denied", msg} }
func NotFound(msg string) *Error        { return &Error{404, "not_found", msg} }
func Conflict(msg string) *Error        { return &Error{409, "already_exists", msg} }
func Internal(msg string) *Error        { return &Error{500, "internal", msg} }
func TooManyRequests(msg string) *Error { return &Error{429, "resource_exhausted", msg} }
func BadGateway(msg string) *Error      { return &Error{502, "upstream_unavailable", msg} }
