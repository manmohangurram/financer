package httpserver

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/mohan9182/financer/auth"
	"github.com/mohan9182/financer/services"
)

type API struct {
	authSvc      *services.AuthService
	userSvc      *services.UserService
	account      *services.AccountService
	category     *services.CategoryService
	txn          *services.TransactionService
	transfer     *services.TransferService
	rule         *services.RuleService
	transferRule *services.TransferRuleService
	invest       *services.InvestmentService
	secret       string
}

func NewAPI(authSvc *services.AuthService, userSvc *services.UserService, account *services.AccountService, category *services.CategoryService, txn *services.TransactionService, transfer *services.TransferService, rule *services.RuleService, transferRule *services.TransferRuleService, invest *services.InvestmentService, jwtSecret string) *API {
	return &API{authSvc: authSvc, userSvc: userSvc, account: account, category: category, txn: txn, transfer: transfer, rule: rule, transferRule: transferRule, invest: invest, secret: jwtSecret}
}

func (a *API) Handler() http.Handler {
	mux := http.NewServeMux()
	a.route(mux, "POST", "/api/auth/signup", true, a.authSignup)
	a.route(mux, "POST", "/api/auth/login", true, a.authLogin)
	a.route(mux, "POST", "/api/auth/refresh", true, a.authRefreshToken)

	a.route(mux, "GET", "/api/me/profile", false, a.getProfile)
	a.route(mux, "PUT", "/api/me/profile", false, a.updateProfile)
	a.route(mux, "POST", "/api/me/password", false, a.changePassword)
	a.route(mux, "POST", "/api/me/logout-all", false, a.logoutAll)
	a.route(mux, "POST", "/api/me/avatar", false, a.uploadAvatar)

	a.route(mux, "GET", "/api/accounts", false, a.listAccounts)
	a.route(mux, "POST", "/api/accounts", false, a.createAccount)
	a.route(mux, "PUT", "/api/accounts/{id}", false, a.updateAccount)
	a.route(mux, "DELETE", "/api/accounts/{id}", false, a.deleteAccount)

	a.route(mux, "GET", "/api/categories", false, a.listCategories)
	a.route(mux, "POST", "/api/categories", false, a.createCategories)
	a.route(mux, "PUT", "/api/categories", false, a.updateCategories)
	a.route(mux, "DELETE", "/api/categories", false, a.deleteCategories)

	a.route(mux, "GET", "/api/transactions", false, a.listTransactions)
	a.route(mux, "POST", "/api/transactions", false, a.createTransactions)
	a.route(mux, "PUT", "/api/transactions", false, a.updateTransactions)
	a.route(mux, "DELETE", "/api/transactions", false, a.deleteTransactions)

	a.route(mux, "POST", "/api/transfer-links", false, a.linkTransfers)
	a.route(mux, "DELETE", "/api/transfer-links", false, a.unlinkTransfers)
	a.route(mux, "POST", "/api/transfer-links/counterpart", false, a.createCounterpart)

	a.route(mux, "GET", "/api/rules", false, a.listRules)
	a.route(mux, "POST", "/api/rules", false, a.createRule)
	a.route(mux, "PUT", "/api/rules/{id}", false, a.updateRule)
	a.route(mux, "DELETE", "/api/rules/{id}", false, a.deleteRule)
	a.route(mux, "POST", "/api/rules/preview", false, a.previewRule)
	a.route(mux, "POST", "/api/rules/{id}/run", false, a.runRule)

	a.route(mux, "GET", "/api/investments", false, a.listInvestments)
	a.route(mux, "POST", "/api/investments", false, a.createInvestment)
	a.route(mux, "PUT", "/api/investments/{id}", false, a.updateInvestment)
	a.route(mux, "DELETE", "/api/investments/{id}", false, a.deleteInvestment)
	a.route(mux, "GET", "/api/investments/{id}/lots", false, a.listLots)
	a.route(mux, "POST", "/api/investments/{id}/lots", false, a.addLot)
	a.route(mux, "DELETE", "/api/investments/{id}/lots/{lotId}", false, a.deleteLot)
	a.route(mux, "GET", "/api/investments/{id}/price-history", false, a.priceHistory)
	a.route(mux, "GET", "/api/investments/search", false, a.searchSymbols)
	a.route(mux, "POST", "/api/investments/refresh-prices", false, a.refreshPrices)
	a.route(mux, "GET", "/api/portfolio/summary", false, a.getPortfolioSummary)

	return cors(a.auth(mux))
}

// statusBody lets handlers signal a non-200 status (201 on create, 204 on delete).
type statusBody struct {
	status int
	body   any
}

func created(v any) statusBody {
	return statusBody{status: http.StatusCreated, body: v}
}

func noContent() statusBody {
	return statusBody{status: http.StatusNoContent}
}

// handler decodes the body and writes a JSON result/error.
type handler func(ctx context.Context, userID string, r *http.Request) (any, error)

func (a *API) route(mux *http.ServeMux, method, path string, public bool, h handler) {
	mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("http request", "method", method, "path", path)
		uid := ""
		if v, ok := r.Context().Value(auth.UserIDKey).(string); ok {
			uid = v
		}
		out, err := h(r.Context(), uid, r)
		if err != nil {
			writeError(w, err)
			return
		}
		status := http.StatusOK
		if sb, ok := out.(statusBody); ok {
			status = sb.status
			out = sb.body
		}
		writeJSON(w, status, out)
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
	if status == http.StatusNoContent {
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	code := 500
	name := "internal"
	if apiErr, ok := err.(*services.APIError); ok {
		code = apiErr.Status
		name = statusName(code)
	} else {
		slog.Error("handler error", "err", err)
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
