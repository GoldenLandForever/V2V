package store

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// TaskRecord 用户任务记录
type TaskRecord struct {
	TaskID   string                 `json:"task_id"`
	TaskType string                 `json:"task_type"` // V2T, T2I, I2V
	Status   string                 `json:"status"`
	Data     map[string]interface{} `json:"data"`
	Cursor   string                 `json:"cursor,omitempty"`
}

// UserHistoryPage 分页结果
type UserHistoryPage struct {
	Tasks      []TaskRecord `json:"tasks"`
	NextCursor string       `json:"next_cursor"` // 下一页游标，空表示无更多数据
	HasMore    bool         `json:"has_more"`
	Total      int64        `json:"total"` // 当前页任务数
	PageSize   int          `json:"page_size"`
}

// GetUserTaskHistory 根据用户ID从Redis获取任务历史，支持游标分页
// userID: 用户ID
// cursor: 分页游标，首次请求传空字符串
// pageSize: 每页返回的任务数，建议 10-50
// Returns: 当前页任务列表、下一页游标、是否有更多、错误
func GetUserTaskHistory(userID uint64, cursor string, pageSize int) (*UserHistoryPage, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10 // 默认 10
	}

	// 使用 SCAN 命令扫描所有匹配 user:userID:*task:* 的key
	userPrefix := fmt.Sprintf("user:%d:", userID)
	pattern := userPrefix + "*task:*"

	// 将光标转换为整数（Redis SCAN 的游标）
	var scanCursor uint64
	if cursor != "" {
		c, err := strconv.ParseUint(cursor, 10, 64)
		if err != nil {
			scanCursor = 0
		} else {
			scanCursor = c
		}
	}

	// 扫描 Redis key
	var allKeys []string
	for {
		keys, newCursor, err := Client.Scan(scanCursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan redis keys: %v", err)
		}
		allKeys = append(allKeys, keys...)
		scanCursor = newCursor
		if scanCursor == 0 {
			break
		}
	}

	// 解析任务记录
	tasks := make([]TaskRecord, 0, len(allKeys))
	for _, key := range allKeys {
		task, err := parseTaskFromKey(key, userID)
		if err != nil {
			continue // 解析失败的key跳过
		}
		tasks = append(tasks, task)
	}

	// 按任务ID排序（降序，最新的在前）
	sort.Slice(tasks, func(i, j int) bool {
		taskIDI, _ := strconv.ParseUint(tasks[i].TaskID, 10, 64)
		taskIDJ, _ := strconv.ParseUint(tasks[j].TaskID, 10, 64)
		return taskIDI > taskIDJ // 降序
	})

	// 应用游标分页
	startIdx := 0
	if cursor != "" {
		// 从光标位置开始
		for i, task := range tasks {
			if task.Cursor == cursor {
				startIdx = i + 1
				break
			}
		}
	}

	endIdx := startIdx + pageSize
	hasMore := endIdx < len(tasks)
	if endIdx > len(tasks) {
		endIdx = len(tasks)
	}

	pageItems := tasks[startIdx:endIdx]
	nextCursor := ""
	if hasMore && endIdx > 0 {
		nextCursor = pageItems[len(pageItems)-1].TaskID
	}

	return &UserHistoryPage{
		Tasks:      pageItems,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Total:      int64(len(pageItems)),
		PageSize:   pageSize,
	}, nil
}

// parseTaskFromKey 从Redis key解析任务信息
func parseTaskFromKey(key string, userID uint64) (TaskRecord, error) {
	prefixV2T := fmt.Sprintf("user:%d:v2ttask:", userID)
	prefixT2I := fmt.Sprintf("user:%d:t2itask:", userID)
	prefixI2V := fmt.Sprintf("user:%d:i2vtaskstatus:", userID)

	var taskID, taskType string

	if strings.HasPrefix(key, prefixV2T) {
		taskID = strings.TrimPrefix(key, prefixV2T)
		taskType = "V2T"
	} else if strings.HasPrefix(key, prefixT2I) {
		taskID = strings.TrimPrefix(key, prefixT2I)
		taskType = "T2I"
	} else if strings.HasPrefix(key, prefixI2V) {
		taskID = strings.TrimPrefix(key, prefixI2V)
		taskType = "I2V"
	} else {
		return TaskRecord{}, fmt.Errorf("unknown key format: %s", key)
	}

	// 从Redis获取任务数据
	data, err := Client.HGetAll(key).Result()
	if err != nil {
		return TaskRecord{}, err
	}

	status := data["status"]
	if status == "" {
		status = "unknown"
	}

	// 将map[string]string转换为map[string]interface{}
	dataInterface := make(map[string]interface{})
	for k, v := range data {
		dataInterface[k] = v
	}

	return TaskRecord{
		TaskID:   taskID,
		TaskType: taskType,
		Status:   status,
		Data:     dataInterface,
		Cursor:   taskID, // 用任务ID作为游标
	}, nil
}

// GetUserTaskHistoryV2 另一种分页方案：基于偏移量的分页（更简单但可能不如游标稳定）
// userID: 用户ID
// offset: 偏移量（第几个任务开始）
// pageSize: 每页大小
func GetUserTaskHistoryV2(userID uint64, offset int, pageSize int) (*UserHistoryPage, error) {
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 10
	}
	if offset < 0 {
		offset = 0
	}

	userPrefix := fmt.Sprintf("user:%d:", userID)
	pattern := userPrefix + "*task:*"

	// 一次性获取所有 key
	keys, _, err := Client.Scan(0, pattern, 10000).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to scan redis keys: %v", err)
	}

	// 解析任务记录
	tasks := make([]TaskRecord, 0, len(keys))
	for _, key := range keys {
		task, err := parseTaskFromKey(key, userID)
		if err != nil {
			continue
		}
		tasks = append(tasks, task)
	}

	// 按任务ID降序排序
	sort.Slice(tasks, func(i, j int) bool {
		taskIDI, _ := strconv.ParseUint(tasks[i].TaskID, 10, 64)
		taskIDJ, _ := strconv.ParseUint(tasks[j].TaskID, 10, 64)
		return taskIDI > taskIDJ
	})

	// 分页计算
	total := len(tasks)
	endIdx := offset + pageSize
	hasMore := endIdx < total

	if endIdx > total {
		endIdx = total
	}
	if offset > total {
		offset = total
	}

	pageItems := tasks[offset:endIdx]

	nextCursor := ""
	if hasMore {
		nextCursor = strconv.Itoa(offset + pageSize)
	}

	return &UserHistoryPage{
		Tasks:      pageItems,
		NextCursor: nextCursor,
		HasMore:    hasMore,
		Total:      int64(len(pageItems)),
		PageSize:   pageSize,
	}, nil
}
