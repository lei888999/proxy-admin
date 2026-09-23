package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"singbox-admin/internal/inbound"
)

// respondMutation answers a write endpoint.
//
// A *inbound.ApplyError means the database change COMMITTED and only the
// sing-box apply step failed. Reporting that as an error was actively harmful:
// the UI said "创建失败" while the row existed, so the retry hit a duplicate-tag
// conflict and the user had no way to tell the two states apart. Those are
// answered 200 with a configApplyError field instead; anything else is a real
// failure and goes to the caller's error mapper.
func respondMutation(c *gin.Context, entity any, err error, onError func(*gin.Context, error)) {
	if err == nil {
		c.JSON(http.StatusOK, entity)
		return
	}
	var ae *inbound.ApplyError
	if !errors.As(err, &ae) {
		onError(c, err)
		return
	}
	body := map[string]any{}
	if b, merr := json.Marshal(entity); merr == nil {
		_ = json.Unmarshal(b, &body)
	}
	body["configApplyError"] = ae.Error()
	c.JSON(http.StatusOK, body)
}

// okBody is the payload for mutations that return no entity.
func okBody() map[string]any { return map[string]any{"ok": true} }
