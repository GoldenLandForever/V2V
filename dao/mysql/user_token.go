package mysql

import (
	"V2V/models"
	"database/sql"
	"errors"
	"fmt"
)

// UserToken 用户Token信息

// GetUserToken 根据用户ID获取Token信息
func GetUserToken(userID uint64) (*models.UserToken, error) {
	userToken := &models.UserToken{}
	sqlStr := "SELECT user_id, tokens, vip_level, created_at, updated_at FROM t_user_tokens WHERE user_id = ?"
	err := Db.Get(userToken, sqlStr, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("user token not found")
		}
		return nil, err
	}
	return userToken, nil
}

// InitUserToken 为新用户初始化Token记录（初始100 token）
func InitUserToken(userID uint64, initialTokens int64) error {
	sqlStr := `INSERT INTO t_user_tokens (user_id, tokens, vip_level, created_at, updated_at) 
	           VALUES (?, ?, 0, NOW(), NOW()) 
	           ON DUPLICATE KEY UPDATE tokens = tokens + ?`
	_, err := Db.Exec(sqlStr, userID, initialTokens, initialTokens)
	return err
}

// AddTokens 给用户添加Token（充值、奖励等）
func AddTokens(userID uint64, amount int64) error {
	if amount == 0 {
		return errors.New("amount must be greater than 0")
	}
	sqlStr := "UPDATE t_user_tokens SET tokens = tokens + ?, updated_at = NOW() WHERE user_id = ?"
	result, err := Db.Exec(sqlStr, amount, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}
	return nil
}

// DeductTokensForTask 在一个事务内对 t2i_tasks 和 t_user_tokens 同时加行锁，
// 当且仅当 t2i_tasks.status = 'succeed' 时，将任务状态更新为 'tokenpay' 并扣除用户 token。
// 该操作为原子操作，保证幂等性（重复或并发请求只有第一次会成功扣费并修改任务状态）。
func DeductTokensForTask(userID uint64, taskID uint64, amount int64) (bool, int64, error) {
	if amount == 0 {
		return false, 0, errors.New("amount must be greater than 0")
	}

	// 1. 查询任务当前状态
	var currentStatus string
	err := Db.Get(&currentStatus, "SELECT status FROM t2i_tasks WHERE task_id = ?", taskID)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, 0, errors.New("task not found")
		}
		return false, 0, fmt.Errorf("failed to query task: %v", err)
	}

	// 2. 检查状态（支持幂等性）
	if currentStatus != "succeed" {
		if currentStatus == "tokenpay" {
			// 已经是付费状态，幂等性返回成功
			var balance int64
			err := Db.Get(&balance, "SELECT tokens FROM t_user_tokens WHERE user_id = ?", userID)
			if err != nil {
				return false, 0, err
			}
			return true, balance, nil
		}
		return false, 0, fmt.Errorf("task status not eligible: %s", currentStatus)
	}

	// 3. 开启事务（保证两个更新的原子性）
	tx, err := Db.Beginx()
	if err != nil {
		return false, 0, fmt.Errorf("failed to begin transaction: %v", err)
	}
	defer tx.Rollback()

	// 4. 先扣除Token
	result, err := tx.Exec(`
        UPDATE t_user_tokens 
        SET tokens = tokens - ?, updated_at = NOW() 
        WHERE user_id = ?`,
		amount, userID)
	if err != nil {
		return false, 0, fmt.Errorf("failed to deduct tokens: %v", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, 0, err
	}

	// 5. 更新任务状态（乐观锁：只有当前是succeed状态才更新）
	result, err = tx.Exec(`
        UPDATE t2i_tasks 
        SET status = 'tokenpay', updated_at = NOW() 
        WHERE task_id = ? AND status = ?`,
		taskID, "succeed")
	if err != nil {
		return false, 0, fmt.Errorf("failed to update task status: %v", err)
	}

	rows, err = result.RowsAffected()
	if err != nil {
		return false, 0, err
	}

	if rows == 0 {
		// 并发问题
		return true, 0, nil
	}

	// 6. 提交事务
	if err = tx.Commit(); err != nil {
		return false, 0, fmt.Errorf("failed to commit transaction: %v", err)
	}

	// 7. 查询最新余额
	var newBalance int64
	err = Db.Get(&newBalance, "SELECT tokens FROM t_user_tokens WHERE user_id = ?", userID)
	if err != nil {
		return true, 0, fmt.Errorf("deduct success but failed to get balance: %v", err)
	}

	return true, newBalance, nil
}

// SetVIPLevel 更新用户VIP等级
func SetVIPLevel(userID uint64, vipLevel uint8) error {
	sqlStr := "UPDATE t_user_tokens SET vip_level = ?, updated_at = NOW() WHERE user_id = ?"
	result, err := Db.Exec(sqlStr, vipLevel, userID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return errors.New("user not found")
	}
	return nil
}
