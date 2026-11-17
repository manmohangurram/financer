package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
)

func (a *API) listKeys(c *gin.Context) {
	keys, err := a.keySvc.List(c.GetString(userIDKey))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, keys)
}

func (a *API) createKey(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqCreateKey
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	out, err := a.keySvc.Create(uid, in.Name, in.ExpiresInDays, in.Scope)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, out)
}

func (a *API) deleteKey(c *gin.Context) {
	if err := a.keySvc.Delete(c.GetString(userIDKey), c.Param("id")); err != nil {
		writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}
