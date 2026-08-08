package httpserver

import (
	"context"
	"net/http"

	"github.com/mohan9182/financer/api"
)

// --- auth ---

type reqAuth struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	RefreshToken string `json:"refreshToken"`
}

// wireAuth matches the frontend auth store contract.
type wireAuth struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	UserId       string `json:"userId"`
	Email        string `json:"email"`
	Name         string `json:"name"`
}

func authWire(a *api.AuthResponse) wireAuth {
	return wireAuth{
		AccessToken:  a.AccessToken,
		RefreshToken: a.RefreshToken,
		UserId:       a.UserId,
		Email:        a.Email,
		Name:         a.Name,
	}
}

func (a *API) authSignup(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAuth
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.authSvc.Signup(ctx, &api.SignupRequest{
		Email: in.Email, Password: in.Password, Name: in.Name,
	})
	if err != nil {
		return nil, err
	}
	return created(authWire(out)), nil
}

func (a *API) authLogin(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAuth
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.authSvc.Login(ctx, &api.LoginRequest{Email: in.Email, Password: in.Password})
	if err != nil {
		return nil, err
	}
	return authWire(out), nil
}

func (a *API) authRefreshToken(ctx context.Context, _ string, r *http.Request) (any, error) {
	var in reqAuth
	if err := decodeBody(r.Body, &in); err != nil {
		return nil, err
	}
	out, err := a.authSvc.RefreshToken(ctx, &api.RefreshTokenRequest{RefreshToken: in.RefreshToken})
	if err != nil {
		return nil, err
	}
	return authWire(out), nil
}
