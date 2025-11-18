package controller

import (
	"V2V/dao/mysql"

	"github.com/gin-gonic/gin"
)

// GetUserTokenInfo 获取用户 Token 信息
// @Summary 获取用户Token信息
// @Description 查询用户当前的 Token 余额和 VIP 等级
// @Tags Token
// @Accept json
// @Produce json
// @Param user_id path uint64 true "用户ID"
// @Success 200 {object} mysql.UserToken "user token info"
// @Failure 400 {object} map[string]string "invalid user_id"
// @Failure 404 {object} map[string]string "user token not found"
// @Failure 500 {object} map[string]string "server error"
// @Router /api/v1/token/info/:user_id [get]
func GetUserTokenInfo(c *gin.Context) {
	_UserID, ok := c.Get("user_id")
	if !ok {
		ResponseError(c, 400)
		return
	}

	userToken, err := mysql.GetUserToken(_UserID.(uint64))
	if err != nil {
		ResponseError(c, 500)
		return
	}
	ResponseSuccess(c, userToken)
}
