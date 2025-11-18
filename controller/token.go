package controller

import (
	"V2V/dao/mysql"
	"strconv"

	"github.com/gin-gonic/gin"
)

// DeductTokenRequest T2I token 扣除请求
type DeductTokenRequest struct {
	TaskID     uint64 `json:"task_id" binding:"required"`
	UserID     uint64 `json:"user_id" binding:"required"`
	TokenCount uint64 `json:"token_count" binding:"required"`
}

// DeductTokenResponse token 扣除响应
type DeductTokenResponse struct {
	Success         bool   `json:"success"`
	Message         string `json:"message"`
	RemainingTokens uint64 `json:"remaining_tokens,omitempty"`
}

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
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		ResponseError(c, 400)
		return
	}

	userToken, err := mysql.GetUserToken(userID)

	ResponseSuccess(c, userToken)
}
