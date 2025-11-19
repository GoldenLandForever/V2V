package logic

import (
	"V2V/dao/mysql"
	"database/sql"
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

// getT2ITaskStatus 获取 T2I 任务的状态
// 用于确保幂等性：只有在任务状态为 pending 时才能扣除 Token
func getT2ITaskStatus(taskID uint64) (string, error) {
	var status string
	db := mysql.Db
	sqlStr := "SELECT status FROM t2i_tasks WHERE task_id = ?"
	err := db.Get(&status, sqlStr, taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return "", err
		}
		return "", err
	}
	return status, nil
}

// InitUserTokensHandler 初始化用户Token（新用户注册时调用）
// @Summary 初始化新用户Token
// @Description 为新注册用户初始化 Token 记录（初始100 token）
// @Tags Token
// @Accept json
// @Produce json
// @Param user_id path uint64 true "用户ID"
// @Success 200 {object} map[string]string "success"
// @Failure 400 {object} map[string]string "invalid user_id"
// @Failure 500 {object} map[string]string "server error"
// @Router /api/v1/token/init/:user_id [post]
func InitUserTokensHandler(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 64)
	if err != nil {
		return
	}

	const initialTokens = 100
	err = mysql.InitUserToken(userID, initialTokens)
	if err != nil {
		return
	}

}
