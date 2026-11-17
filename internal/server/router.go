// Package server builds the Gin router: middleware (CORS, auth, error
// envelope), route groups, and the SPA fallback. It mirrors the Rust
// backend's http layer.
package server

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/accountservice"
	"github.com/mohan9182/financer/internal/auth"
	"github.com/mohan9182/financer/internal/authservice"
	"github.com/mohan9182/financer/internal/categoryservice"
	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/keyservice"
	"github.com/mohan9182/financer/internal/mailservice"
	mcpserver "github.com/mohan9182/financer/internal/mcp"
	"github.com/mohan9182/financer/internal/model"
	"github.com/mohan9182/financer/internal/ruleservice"
	"github.com/mohan9182/financer/internal/transactionservice"
	"github.com/mohan9182/financer/internal/transferservice"
	"github.com/mohan9182/financer/internal/userservice"
)

const userIDKey = "user_id"

// Deps is everything the HTTP layer needs at boot.
type Deps struct {
	Auth          *authservice.Service
	User          *userservice.Service
	Account       *accountservice.Service
	Category      *categoryservice.Service
	Transaction   *transactionservice.Service
	Transfer      *transferservice.Service
	Rule          *ruleservice.Service
	TransferRule  *ruleservice.TransferRuleService
	Mail          *mailservice.Service
	Keys          *keyservice.Service
	JWTSecret     string
	StaticDir     string
	AvatarDir     string
	DomainURL     string
	AllowInsecure bool
}

type API struct {
	authSvc       *authservice.Service
	userSvc       *userservice.Service
	accountSvc    *accountservice.Service
	categorySvc   *categoryservice.Service
	txnSvc        *transactionservice.Service
	transferSvc   *transferservice.Service
	ruleSvc       *ruleservice.Service
	transferRule  *ruleservice.TransferRuleService
	mailSvc       *mailservice.Service
	keySvc        *keyservice.Service
	mcpServices   mcpserver.Services
	jwtSecret     string
	staticDir     string
	avatarDir     string
	domainURL     string
	allowInsecure bool
}

func New(d Deps) *API {
	return &API{
		authSvc:      d.Auth,
		userSvc:      d.User,
		accountSvc:   d.Account,
		categorySvc:  d.Category,
		txnSvc:       d.Transaction,
		transferSvc:  d.Transfer,
		ruleSvc:      d.Rule,
		transferRule: d.TransferRule,
		mailSvc:      d.Mail,
		keySvc:       d.Keys,
		mcpServices: mcpserver.Services{
			Accounts:      d.Account,
			Transactions:  d.Transaction,
			Categories:    d.Category,
			Rules:         d.Rule,
			Transfers:     d.Transfer,
			TransferRules: d.TransferRule,
		},
		jwtSecret:     d.JWTSecret,
		staticDir:     d.StaticDir,
		avatarDir:     d.AvatarDir,
		domainURL:     d.DomainURL,
		allowInsecure: d.AllowInsecure,
	}
}

func (a *API) Handler() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(recoveryMiddleware())
	r.Use(requestLogger())
	r.Use(cors())

	// Public auth routes.
	authGroup := r.Group("/api/auth")
	{
		authGroup.POST("/signup", a.authSignup)
		authGroup.POST("/login", a.authLogin)
		authGroup.POST("/refresh", a.authRefreshToken)
	}

	// Authenticated routes.
	api := r.Group("/api", a.authRequired())
	{
		api.GET("/me/profile", a.getProfile)
		api.PUT("/me/profile", a.updateProfile)
		api.POST("/me/password", a.changePassword)
		api.POST("/me/logout-all", a.logoutAll)
		api.POST("/me/avatar", a.avatar)

		api.GET("/me/keys", a.listKeys)
		api.POST("/me/keys", a.createKey)
		api.DELETE("/me/keys/:id", a.deleteKey)

		api.GET("/accounts", a.listAccounts)
		api.POST("/accounts", a.createAccount)
		api.PUT("/accounts/:id", a.updateAccount)
		api.DELETE("/accounts/:id", a.deleteAccount)

		api.GET("/categories", a.listCategories)
		api.POST("/categories", a.createCategories)
		api.PUT("/categories", a.updateCategories)
		api.DELETE("/categories", a.deleteCategories)

		api.GET("/transactions", a.listTransactions)
		api.POST("/transactions", a.createTransactions)
		api.PUT("/transactions", a.updateTransactions)
		api.DELETE("/transactions", a.deleteTransactions)

		api.POST("/transfer-links", a.linkTransfers)
		api.DELETE("/transfer-links", a.unlinkTransfers)
		api.POST("/transfer-links/counterpart", a.createTransferCounterpart)

		api.GET("/rules", a.listRules)
		api.POST("/rules", a.createRule)
		api.PUT("/rules/:id", a.updateRule)
		api.DELETE("/rules/:id", a.deleteRule)
		api.POST("/rules/preview", a.previewRules)
		api.POST("/rules/:id/run", a.runRule)

		api.GET("/dashboard", a.dashboard)
		api.GET("/spending", a.spending)

		mail := api.Group("/mail", a.requireHTTPS())
		{
			mail.GET("/accounts", a.listMailAccounts)
			mail.POST("/accounts", a.createMailAccount)
			mail.DELETE("/accounts/:id", a.deleteMailAccount)
			mail.POST("/accounts/:id/poll", a.pollMailAccount)
			mail.GET("/messages", a.listMailMessages)
			mail.POST("/messages", a.mailAct)
			mail.GET("/rules", a.listMailRules)
			mail.POST("/rules", a.createMailRule)
			mail.PUT("/rules/:id", a.updateMailRule)
			mail.DELETE("/rules/:id", a.deleteMailRule)
		}
	}

	// Avatars are served without auth (mirrors Rust).
	r.GET("/avatars/:name", a.avatarFile)

	// API description + Swagger UI.
	r.GET("/openapi.json", a.openAPISpec)
	r.GET("/docs", a.swaggerUI)

	// MCP streamable-HTTP server (authenticated by API key, not JWT).
	r.Any("/mcp", gin.WrapH(a.mcpHandler()))

	// SPA fallback for everything else.
	r.NoRoute(a.spa)

	return r
}

// --- middleware ---

// recoveryMiddleware turns a handler panic into a logged 500 instead of
// terminating the process or printing an unstructured stack trace.
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("panic recovered",
					"panic", fmt.Sprint(recovered),
					"method", c.Request.Method,
					"path", c.Request.URL.Path,
					"stack", string(debug.Stack()),
				)
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
					"code": "internal", "message": "internal error",
				})
			}
		}()
		c.Next()
	}
}

// requestLogger records one structured line per API/MCP request. Static assets
// and the SPA shell are skipped; their volume would bury operational events.
func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		path := c.Request.URL.Path
		if !strings.HasPrefix(path, "/api") && path != "/mcp" && !strings.HasPrefix(path, "/mcp") {
			c.Next()
			return
		}
		start := time.Now()
		c.Next()

		attrs := []any{
			"method", c.Request.Method,
			"path", path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
			"bytes", c.Writer.Size(),
			"ip", c.ClientIP(),
		}
		if uid := c.GetString(userIDKey); uid != "" {
			attrs = append(attrs, "user_id", uid)
		}
		switch status := c.Writer.Status(); {
		case status >= 500:
			slog.Error("http request", attrs...)
		case status >= 400:
			slog.Warn("http request", attrs...)
		default:
			slog.Info("http request", attrs...)
		}
	}
}

// cors sets CORS headers and answers preflight (permissive, mirrors Rust).
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}

// authRequired validates the bearer token and stashes the user id.
func (a *API) authRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		hdr := c.GetHeader("Authorization")
		const prefix = "Bearer "
		if len(hdr) < len(prefix) || !strings.EqualFold(hdr[:len(prefix)], prefix) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthenticated",
				"message": "missing or invalid Authorization header",
			})
			return
		}
		claims, err := auth.New(a.jwtSecret).Validate(hdr[len(prefix):])
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"code":    "unauthenticated",
				"message": "invalid token",
			})
			return
		}
		c.Set(string(userIDKey), claims.UserID)
		c.Next()
	}
}

// writeError maps an error to the {code, message} envelope with the right status.
func writeError(c *gin.Context, err error) {
	code := 500
	name := "internal"
	msg := err.Error()
	if apiErr, ok := err.(*errors.Error); ok {
		code = apiErr.Status
		name = apiErr.Code
		msg = apiErr.Message
	}
	// 5xx responses hide the cause from the client; log it so the server log
	// explains what actually failed.
	if code >= 500 {
		slog.Error("request failed",
			"method", c.Request.Method, "path", c.Request.URL.Path,
			"status", code, "err", err)
	}
	c.JSON(code, gin.H{"code": name, "message": msg})
}

// --- auth handlers ---

type reqAuth struct {
	Email        string `json:"email"`
	Password     string `json:"password"`
	Name         string `json:"name"`
	RefreshToken string `json:"refreshToken"`
}

func (a *API) authSignup(c *gin.Context) {
	var in reqAuth
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.authSvc.Signup(in.Email, in.Password, in.Name)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) authLogin(c *gin.Context) {
	var in reqAuth
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.authSvc.Login(in.Email, in.Password)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) authRefreshToken(c *gin.Context) {
	var in reqAuth
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.authSvc.RefreshToken(in.RefreshToken)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// --- user settings handlers ---

func (a *API) getProfile(c *gin.Context) {
	out, err := a.userSvc.Profile(c.GetString(string(userIDKey)))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type reqProfile struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatarUrl"`
}

func (a *API) updateProfile(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in reqProfile
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.userSvc.UpdateProfile(uid, in.Name, in.Email, in.AvatarURL)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

type reqPassword struct {
	CurrentPassword string `json:"currentPassword"`
	NewPassword     string `json:"newPassword"`
}

func (a *API) changePassword(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in reqPassword
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.userSvc.ChangePassword(uid, in.CurrentPassword, in.NewPassword)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) logoutAll(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	if err := a.userSvc.LogoutAll(uid); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.BulkResult{Success: true, Message: "all sessions logged out"})
}

func (a *API) avatar(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	file, err := c.FormFile("file")
	if err != nil {
		writeError(c, errors.BadRequest("missing 'file' field"))
		return
	}
	f, err := file.Open()
	if err != nil {
		writeError(c, errors.BadRequest("failed to read avatar"))
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		writeError(c, errors.BadRequest("failed to read avatar"))
		return
	}
	out, err := a.userSvc.SaveAvatar(uid, data)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

// --- accounts handlers ---

func (a *API) listAccounts(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	items, err := a.accountSvc.List(uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"accounts": items})
}

func (a *API) createAccount(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in model.ReqAccount
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	if !validAccountType(in.AccountType) {
		writeError(c, errors.BadRequest("invalid account type"))
		return
	}
	out, err := a.accountSvc.Create(uid, in.BankName, in.Nickname, string(in.AccountType), in.EndingNumbers)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) updateAccount(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	id := c.Param("id")
	var in model.ReqAccount
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	if !validAccountType(in.AccountType) {
		writeError(c, errors.BadRequest("invalid account type"))
		return
	}
	out, err := a.accountSvc.Update(uid, id, in.BankName, in.Nickname, string(in.AccountType), in.EndingNumbers)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) deleteAccount(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	id := c.Param("id")
	if err := a.accountSvc.Delete(uid, id); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func validAccountType(t model.AccountType) bool {
	switch t {
	case model.AccountTypeCurrent, model.AccountTypeSavings, model.AccountTypeLoan, model.AccountTypeCreditCard:
		return true
	}
	return false
}

// --- categories handlers ---

func (a *API) listCategories(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	pageSize := parseInt(c.Query("pageSize"), 0)
	items, nextToken, err := a.categorySvc.List(uid, int64(pageSize), c.Query("pageToken"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"categories": items, "nextPageToken": nextToken})
}

type reqCategory struct {
	IDs        []string `json:"ids"`
	Categories []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"categories"`
}

func (a *API) createCategories(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in reqCategory
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	names := make([]string, 0, len(in.Categories))
	for _, cat := range in.Categories {
		names = append(names, cat.Name)
	}
	res := a.categorySvc.Create(uid, names)
	c.JSON(http.StatusCreated, res)
}

func (a *API) updateCategories(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in reqCategory
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	pairs := make([][2]string, 0, len(in.Categories))
	for _, cat := range in.Categories {
		pairs = append(pairs, [2]string{cat.ID, cat.Name})
	}
	res := a.categorySvc.Update(uid, pairs)
	c.JSON(http.StatusOK, res)
}

func (a *API) deleteCategories(c *gin.Context) {
	uid := c.GetString(string(userIDKey))
	var in reqCategory
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	res := a.categorySvc.Delete(uid, in.IDs)
	if !res.Success {
		writeError(c, errors.BadRequest(res.Message))
		return
	}
	c.Status(http.StatusNoContent)
}

// --- static / SPA fallback ---

const maxAvatarBytes = 5 << 20

func (a *API) avatarFile(c *gin.Context) {
	name := c.Param("name")
	if name == "" || strings.Contains(name, "..") {
		c.Status(http.StatusNotFound)
		return
	}
	data, err := os.ReadFile(filepath.Join(a.avatarDir, name))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	mime := mimeFor(name)
	if mime == "application/octet-stream" {
		// Unknown extensions fall through to a guess by extension.
		switch {
		case strings.HasSuffix(name, ".png"):
			mime = "image/png"
		case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
			mime = "image/jpeg"
		case strings.HasSuffix(name, ".webp"):
			mime = "image/webp"
		}
	}
	c.Data(http.StatusOK, mime, data)
}

func (a *API) spa(c *gin.Context) {
	p := strings.TrimPrefix(c.Request.URL.Path, "/")
	if p != "" && !strings.Contains(p, "..") {
		candidate := filepath.Join(a.staticDir, p)
		if data, err := os.ReadFile(candidate); err == nil {
			c.Data(http.StatusOK, mimeFor(p), data)
			return
		}
	}
	index, err := os.ReadFile(filepath.Join(a.staticDir, "index.html"))
	if err != nil {
		index = []byte("<html><body>Financer</body></html>")
	} else if a.domainURL != "" {
		// Inject the API base script like Rust's read_index.
		script := "<script>window.__API_BASE__=" + quote(a.domainURL) + ";</script>"
		s := string(index)
		if i := strings.Index(s, "</head>"); i >= 0 {
			s = s[:i] + script + s[i:]
		}
		index = []byte(s)
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", index)
}

func quote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func mimeFor(p string) string {
	switch strings.ToLower(path.Ext(p)) {
	case ".js":
		return "text/javascript"
	case ".css":
		return "text/css"
	case ".png":
		return "image/png"
	case ".svg":
		return "image/svg+xml"
	case ".woff2":
		return "font/woff2"
	case ".ico":
		return "image/x-icon"
	case ".map":
		return "application/json"
	}
	return "application/octet-stream"
}

// parseInt is a small helper for query params.
func parseInt(s string, def int) int {
	if s == "" {
		return def
	}
	n := 0
	for _, b := range []byte(s) {
		if b < '0' || b > '9' {
			return def
		}
		n = n*10 + int(b-'0')
	}
	return n
}
