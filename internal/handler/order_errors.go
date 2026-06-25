package handler

import (
	"errors"
	"net/http"

	"recipe-server/internal/service"

	"github.com/gin-gonic/gin"
)

func writeOrderAddError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrDuplicateOrder):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
	case errors.Is(err, service.ErrInvalidMealType):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
	case errors.Is(err, service.ErrNoFamily):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": err.Error()})
	case errors.Is(err, service.ErrRecipeNotInFamily):
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "msg": "菜谱不存在或不属于当前家庭"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "msg": "点菜失败"})
	}
}
