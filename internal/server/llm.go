package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/mohan9182/financer/internal/errors"
	"github.com/mohan9182/financer/internal/model"
)

func (a *API) getLLMSettings(c *gin.Context) {
	settings, err := a.llmSvc.Get(c.GetString(userIDKey))
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (a *API) updateLLMSettings(c *gin.Context) {
	uid := c.GetString(userIDKey)
	var in model.ReqLLMSettings
	if err := c.ShouldBindJSON(&in); err != nil {
		writeError(c, errors.BadRequest(err.Error()))
		return
	}
	settings, err := a.llmSvc.Update(uid, in)
	if err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (a *API) testLLMSettings(c *gin.Context) {
	if err := a.llmSvc.Test(c.Request.Context(), c.GetString(userIDKey)); err != nil {
		writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, model.LLMTestResult{OK: true, Message: "connection ok"})
}
