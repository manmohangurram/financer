package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	mcpserver "github.com/mohan9182/financer/internal/mcp"
	"github.com/mohan9182/financer/internal/openapi"
)

type ctxKey string

const (
	mcpUserKey  ctxKey = "mcp_user_id"
	mcpScopeKey ctxKey = "mcp_scope"
)

// mcpHandler mounts the MCP streamable-HTTP server at /mcp, authenticated by a
// per-user API key (not a JWT).
func (a *API) mcpHandler() http.Handler {
	streamable := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		uid, _ := r.Context().Value(mcpUserKey).(string)
		scope, _ := r.Context().Value(mcpScopeKey).(string)
		return mcpserver.Build(a.mcpServices, uid, scope)
	}, nil)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := bearerToken(r)
		if key == "" {
			mcpUnauthorized(w)
			return
		}
		uid, scope, err := a.keySvc.Authenticate(key)
		if err != nil {
			slog.Warn("mcp authentication failed", "ip", r.RemoteAddr, "err", err)
			mcpUnauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), mcpUserKey, uid)
		ctx = context.WithValue(ctx, mcpScopeKey, scope)
		streamable.ServeHTTP(w, r.WithContext(ctx))
	})
}

func bearerToken(r *http.Request) string {
	hdr := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(hdr) < len(prefix) || !strings.EqualFold(hdr[:len(prefix)], prefix) {
		return ""
	}
	return strings.TrimSpace(hdr[len(prefix):])
}

func mcpUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid or missing API key"})
}

// openAPISpec serves the machine-readable API description.
func (a *API) openAPISpec(c *gin.Context) {
	c.JSON(http.StatusOK, openapi.Spec())
}

// swaggerUI serves a Swagger UI page pointed at /openapi.json.
func (a *API) swaggerUI(c *gin.Context) {
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(swaggerHTML))
}

const swaggerHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>Financer API docs</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js" crossorigin></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>`
