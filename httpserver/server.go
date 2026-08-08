package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/services"
)

type API struct {
	authSvc *services.AuthService
	account *services.AccountService
	secret  string
}

func NewAPI(authSvc *services.AuthService, account *services.AccountService, jwtSecret string) *API {
	return &API{authSvc: authSvc, account: account, secret: jwtSecret}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	a.route(mux, "POST", "/api/auth/signup", true, a.authSignup)
	a.route(mux, "POST", "/api/auth/login", true, a.authLogin)
	a.route(mux, "POST", "/api/auth/refresh", true, a.authRefreshToken)
	a.route(mux, "GET", "/api/me", false, a.authGetMe)

	a.route(mux, "GET", "/api/accounts", false, a.listAccounts)
	a.route(mux, "POST", "/api/accounts", false, a.createAccount)
	a.route(mux, "PUT", "/api/accounts/{id}", false, a.updateAccount)
	a.route(mux, "DELETE", "/api/accounts/{id}", false, a.deleteAccount)

	return cors(a.auth(mux))
}

// handler decodes the body and writes a JSON result/error.
type handler func(ctx context.Context, userID string, r *http.Request) (any, error)

func (a *API) route(mux *http.ServeMux, method, path string, public bool, h handler) {
	mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", method, path)
		uid := ""
		if v, ok := r.Context().Value(auth.UserIDKey).(string); ok {
			uid = v
		}
		out, err := h(r.Context(), uid, r)
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	})
}

func (a *API) auth(next http.Handler) http.Handler {
	public := map[string]bool{
		"/api/auth/signup":  true,
		"/api/auth/login":   true,
		"/api/auth/refresh": true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if public[r.URL.Path] {
			next.ServeHTTP(w, r)
			return
		}
		hdr := r.Header.Get("Authorization")
		const prefix = "Bearer "
		if len(hdr) < len(prefix) || !strings.EqualFold(hdr[:len(prefix)], prefix) {
			writeError(w, services.Unauthorized("missing or invalid Authorization header"))
			return
		}
		claims, err := auth.ValidateToken(hdr[len(prefix):], a.secret)
		if err != nil {
			writeError(w, services.Unauthorized("%v", err))
			return
		}
		ctx := context.WithValue(r.Context(), auth.UserIDKey, claims.UserID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func decodeBody(body io.Reader, dst any) error {
	dec := json.NewDecoder(body)
	if err := dec.Decode(dst); err != nil && err != io.EOF {
		return &services.APIError{Status: 400, Msg: err.Error()}
	}
	return nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := 500
	name := "internal"
	if apiErr, ok := err.(*services.APIError); ok {
		code = apiErr.Status
		name = statusName(code)
	} else {
		log.Printf("handler error: %v", err)
	}
	writeJSON(w, code, map[string]string{"code": name, "message": err.Error()})
}

func statusName(code int) string {
	switch code {
	case 400:
		return "invalid_argument"
	case 401:
		return "unauthenticated"
	case 404:
		return "not_found"
	case 409:
		return "already_exists"
	default:
		return "internal"
	}
}
