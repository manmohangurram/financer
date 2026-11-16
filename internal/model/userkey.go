package model

// UserKey is a masked API key returned by the list endpoint.
type UserKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	KeyPrefix  string  `json:"keyPrefix"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  *string `json:"expiresAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	Scope      string  `json:"scope"`
}

// CreatedUserKey is the create response: the only time the plaintext key is
// shown.
type CreatedUserKey struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Key        string  `json:"key"`
	KeyPrefix  string  `json:"keyPrefix"`
	CreatedAt  string  `json:"createdAt"`
	ExpiresAt  *string `json:"expiresAt"`
	LastUsedAt *string `json:"lastUsedAt"`
	Scope      string  `json:"scope"`
}

// ReqCreateKey is the create-API-key request.
type ReqCreateKey struct {
	Name          string `json:"name"`
	ExpiresInDays int64  `json:"expiresInDays"`
	Scope         string `json:"scope"`
}
