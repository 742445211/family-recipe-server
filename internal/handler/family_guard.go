package handler

import (
	"net/http"

	"recipe-server/internal/middleware"

	"github.com/gin-gonic/gin"
)

// requireFamilyID 校验当前请求已解析出有效家庭 ID（须已加入家庭）。
func requireFamilyID(c *gin.Context) (uint64, bool) {
	fid := middleware.GetFamilyID(c)
	if fid == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "请先加入家庭"})
		return 0, false
	}
	return fid, true
}
