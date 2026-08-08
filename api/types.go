// Package api holds the API request/response types. Hand-written,
// no protobuf. Field shapes mirror what the frontend's fetch clients send/receive.
package api

type SignupRequest struct {
	Email    string
	Password string
	Name     string
}

type LoginRequest struct {
	Email    string
	Password string
}

type RefreshTokenRequest struct {
	RefreshToken string
}

type GetMeRequest struct{}

type AuthResponse struct {
	AccessToken  string
	RefreshToken string
	UserId       string
	Email        string
	Name         string
}

type GetMeResponse struct {
	UserId string
	Email  string
	Name   string
}
