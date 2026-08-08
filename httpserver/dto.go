package httpserver

import "github.com/mohan9182/financer/api"

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

type wireMe struct {
	UserId string `json:"userId"`
	Email  string `json:"email"`
	Name   string `json:"name"`
}
