package server

import (
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
)

// requireHTTPS refuses credential-bearing mail endpoints over plain HTTP.
// Loopback is exempt so local development needs no certificate, and
// FINANCER_ALLOW_INSECURE_HTTP overrides the guard entirely.
func (a *API) requireHTTPS() gin.HandlerFunc {
	return func(c *gin.Context) {
		if a.allowInsecure {
			c.Next()
			return
		}
		if c.Request.TLS != nil ||
			strings.EqualFold(c.GetHeader("X-Forwarded-Proto"), "https") ||
			peerIsLoopback(c.Request.RemoteAddr) {
			c.Next()
			return
		}
		writeError(c, errors.Forbidden("this endpoint requires HTTPS"))
		c.Abort()
	}
}

func peerIsLoopback(remoteAddr string) bool {
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		host = remoteAddr
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// --- mail accounts ---

func (a *API) listMailAccounts(c *gin.Context) {
	uid := c.GetString(userIDKey)
	pageSize := int64(parseInt(c.Query("pageSize"), 0))
	pageToken := c.Query("pageToken")
	if pageSize > 0 || pageToken != "" {
		accounts, next, err := a.mailSvc.ListPage(uid, pageSize, pageToken)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, model.WireMailAccounts{Accounts: accounts, NextPageToken: next})
		return
	}
	accounts, err := a.mailSvc.List(uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WireMailAccounts{Accounts: accounts})
}

func (a *API) createMailAccount(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailAccount
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	account, err := a.mailSvc.Create(uid, in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, account)
}

func (a *API) deleteMailAccount(c *gin.Context) {
	uid := c.GetString(userIDKey)
	if err := a.mailSvc.Delete(uid, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) updateMailAccount(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqUpdateMailAccount
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.mailSvc.Update(uid, c.Param("id"), in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

// pollMailUser starts an asynchronous poll for all of the user's mailboxes.
func (a *API) pollMailUser(c *gin.Context) {
	force := c.Query("force") == "1" || c.Query("force") == "true"
	status := a.mailSvc.PollUserAsync(c.GetString(userIDKey), force)
	c.JSON(http.StatusAccepted, model.WirePollStatus{Status: status})
}

func (a *API) pollMailAccount(c *gin.Context) {
	uid := c.GetString(userIDKey)
	summary, err := a.mailSvc.PollNow(uid, c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WirePollSummary{
		Matched: summary.Matched,
		Stored:  summary.Stored,
		Skipped: summary.Skipped,
	})
}

// --- mail messages ---

func (a *API) listMailMessages(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var status *string
	if s := c.Query("status"); s != "" {
		status = &s
	}
	limit := int64(parseInt(c.Query("pageSize"), 0))
	if limit <= 0 {
		limit = int64(parseInt(c.Query("limit"), 20))
	}
	msgs, total, next, err := a.mailSvc.Messages(uid, status, c.Query("accountId"), c.Query("search"), c.Query("pageToken"), limit)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WireMailMessages{Messages: msgs, Total: total, TotalCount: total, NextPageToken: next})
}

func (a *API) pollMailFull(c *gin.Context) {
	status := a.mailSvc.PollFullAsync(c.GetString(userIDKey))
	c.JSON(http.StatusAccepted, model.WirePollStatus{Status: status})
}

func (a *API) startAIParse(c *gin.Context) {
	st, err := a.mailSvc.StartAIParse(c.GetString(userIDKey))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, st)
}

func (a *API) getAIParseStatus(c *gin.Context) {
	c.JSON(http.StatusOK, a.mailSvc.AIParseStatus(c.GetString(userIDKey)))
}

func (a *API) getMailMessage(c *gin.Context) {
	msg, err := a.mailSvc.Get(c.GetString(userIDKey), c.Param("id"))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, msg)
}

func (a *API) mailAct(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailAction
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	outcome, err := a.mailSvc.Act(uid, in.Messages)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, outcome)
}

// --- mail rules ---

func (a *API) deleteMailMessages(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailDelete
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	var (
		n   int64
		err error
	)
	if in.All {
		n, err = a.mailSvc.DeleteAllMessages(uid)
	} else {
		n, err = a.mailSvc.DeleteMessages(uid, in.IDs)
	}
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, map[string]int64{"deleted": n})
}

func (a *API) listMailRules(c *gin.Context) {
	uid := c.GetString(userIDKey)
	pageSize := int64(parseInt(c.Query("pageSize"), 0))
	pageToken := c.Query("pageToken")
	if pageSize > 0 || pageToken != "" {
		rules, next, err := a.mailSvc.RulesPage(uid, pageSize, pageToken)
		if err != nil {
			writeError(c, err)
			return
		}
		c.JSON(http.StatusOK, model.WireMailRules{Rules: rules, NextPageToken: next})
		return
	}
	rules, err := a.mailSvc.Rules(uid)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.WireMailRules{Rules: rules})
}

func (a *API) createMailRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailRule
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	rule, err := a.mailSvc.CreateRule(uid, in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, rule)
}

func (a *API) updateMailRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailRule
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	if err := a.mailSvc.UpdateRule(uid, c.Param("id"), in); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) validateMailTemplate(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqMailTemplateValidate
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.mailSvc.ValidateTemplate(uid, in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, out)
}

func (a *API) deleteMailRule(c *gin.Context) {
	uid := c.GetString(userIDKey)
	if err := a.mailSvc.DeleteRule(uid, c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
