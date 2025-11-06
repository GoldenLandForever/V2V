package controller

import (
	"V2V/dao/store"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"V2V/pkg/sse"
	"V2V/task"
	"V2V/util"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

//把这个接口改成一次性输入多个参考图和文本提示词，生成多个视频
//参考图和文本提示词从前端传入

func SubmitI2VTask(c *gin.Context) {
	// 请确保您已将 API Key 存储在环境变量 ARK_API_KEY 中
	// 初始化Ark客户端，从环境变量中读取您的API Key
	//测试一下功能
	var t task.I2VTask
	// 从请求体中解析任务参数
	if err := c.ShouldBindJSON(&t); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	key := "user:0:task:" + strconv.FormatInt(int64(t.TaskID), 10)
	// 从redis里找key获得参考图和文本提示词
	hash, err := store.GetRedis().HGetAll(key).Result()
	if err != nil {
		fmt.Printf("failed to get task from redis: %v\n", err)
		return
	}
	prompts := hash["prompt"]
	referenceImages := strings.Split(hash["result"], "|z|k|x|")
	//去掉refereceImages尾部的无用字符串
	referenceImages = referenceImages[:len(referenceImages)-1]
	taskID, err := snowflake.GetID()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	type TaskError struct {
		Index int
		Err   error
	}
	var wg sync.WaitGroup
	errors := make(chan TaskError, len(referenceImages))
	// redis 存储状态，总任务数，成功数，失败数
	redisclient := store.GetRedis()

	statusKey := "user:0:i2vtaskstatus:" + strconv.FormatInt(int64(taskID), 10)
	redisclient.HSet(statusKey, "total", len(referenceImages))
	redisclient.HSet(statusKey, "succeeded", 0)
	redisclient.HSet(statusKey, "failed", 0)

	rabbitMQ, err := queue.GetI2VRabbitMQ()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get I2V message queue"})
		return
	}
	// 提交每个参考图的I2V任务
	for i, refImg := range referenceImages {
		wg.Add(1)
		fmt.Printf("Processing reference image %d: %s\n", i+1, refImg)
		go func(idx int, img string) {
			defer wg.Done()
			var I2Vtask task.I2VTask
			I2Vtask.UserID = 0
			I2Vtask.TaskID = uint64(taskID)
			I2Vtask.Index = idx + 1
			I2Vtask.ImageURL = img
			I2Vtask.Prompt = prompts
			I2Vtask.Priority = 1
			b, err := json.Marshal(I2Vtask)
			if err != nil {
				errors <- TaskError{Index: idx + 1, Err: err}
				return
			}
			err = rabbitMQ.PublishI2VTask(b, I2Vtask.Priority)
			if err != nil {
				errors <- TaskError{Index: idx + 1, Err: err}
				return
			}
			errors <- TaskError{Index: idx + 1, Err: err}
		}(i, refImg)
	}
	wg.Wait()
	close(errors)
	var failed []TaskError
	for taskErr := range errors {
		if taskErr.Err != nil {
			failed = append(failed, taskErr)
		}
	}
	if len(failed) > 0 {
		c.JSON(500, gin.H{
			"error":        "failed to create I2V task",
			"failed_tasks": failed,
		})
		return
	}
	c.JSON(202, gin.H{"task_id": taskID, "status": "submitted"})
}

func GetI2VTaskResult(c *gin.Context) {
	//获取任务结果
	taskID := c.Param("task_id")
	client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
	ctx := c

	req := model.GetContentGenerationTaskRequest{}
	req.ID = taskID

	resp, err := client.GetContentGenerationTask(ctx, req)
	if err != nil {
		fmt.Printf("get content generation task error: %v\n", err)
		return
	}
	// fmt.Printf("%v\n", resp)
	fmt.Println(resp.Content.VideoURL)
	c.JSON(200, gin.H{
		"video_url": resp.Content.VideoURL,
	})
	store.I2VTaskVideoURL(taskID, resp.Content.VideoURL)

}

func I2VCallback(c *gin.Context) {
	taskID := c.Param("task_id")
	var callbackData model.GetContentGenerationTaskResponse
	if err := c.ShouldBindJSON(&callbackData); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	redisclient := store.GetRedis()
	//更新redis中对应任务的状态和视频链接
	key := "user:0:i2vtaskstatus:" + taskID

	// 将任务加入延迟队列（5分钟后检查状态）
	if delayedQueue, err := queue.GetDelayedI2VQueue(); err == nil {
		b := struct {
			TaskID    string `json:"task_id"`
			SubTaskID string `json:"sub_task_id"`
		}{
			TaskID:    taskID,
			SubTaskID: callbackData.ID,
		}
		body, err := json.Marshal(b)
		if err != nil {
			fmt.Printf("Failed to marshal delayed check task: %v\n", err)
		}
		if err := delayedQueue.PublishDelayedCheck(body); err != nil {
			fmt.Printf("Failed to add to delayed queue: %v\n", err)
			// 继续处理，不中断主流程
		}
	}
	// 使用 Redis Lua 脚本做原子比较/更新：
	// 返回值为数组：[succeeded, failed, total, changedFlag]
	// changedFlag: 1 = 新写入或状态变更，0 = 状态未变化
	lua := `
local key = KEYS[1]
local field = ARGV[1]
local new = ARGV[2]
local video_url = ARGV[3]
local total = tonumber(redis.call('HGET', key, 'total') or '0')
local old = redis.call('HGET', key, field)

-- 如果已有终态（succeeded 或 failed），视为不可变，直接返回当前计数（不改变任何东西）
if old == 'succeeded' or old == 'failed' then
	return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed'), tostring(total), 0}
end

-- 首次写入（没有旧状态）
if not old or old == '' then
	redis.call('HSET', key, field, new)
	if new == 'succeeded' then
		redis.call('HINCRBY', key, 'succeeded', 1)
		-- 只存储 video_url（不存其它多余信息），并设置过期
		if video_url and video_url ~= '' then
			redis.call('SET', 'i2v:task:'..field..':video_url', video_url, 'EX', 86400)
		end
	elseif new == 'failed' then
		redis.call('HINCRBY', key, 'failed', 1)
	end
	return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed'), tostring(total), 1}
end

-- 状态未变化
if old == new then
	return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed'), tostring(total), 0}
end

-- old 存在且不是终态（例如 running），允许更新到新状态（succeeded/failed）
redis.call('HSET', key, field, new)
if new == 'succeeded' then
	redis.call('HINCRBY', key, 'succeeded', 1)
	if video_url and video_url ~= '' then
		redis.call('SET', 'i2v:task:'..field..':video_url', video_url, 'EX', 86400)
	end
elseif new == 'failed' then
	redis.call('HINCRBY', key, 'failed', 1)
end
return {redis.call('HGET', key, 'succeeded'), redis.call('HGET', key, 'failed'), tostring(total), 1}
`

	newStatus := strings.ToLower(callbackData.Status)
	// SDK 的 Content 字段包含 VideoURL（当 status 为 succeeded 时必定有值），直接取出即可
	var contentURL string
	if newStatus == "succeeded" {
		contentURL = callbackData.Content.VideoURL
	} else {
		contentURL = ""
	}

	// 执行 Lua 脚本，确保比较/更新/计数为原子操作
	res, err := redisclient.Eval(lua, []string{key}, callbackData.ID, newStatus, contentURL).Result()
	if err != nil {
		fmt.Printf("redis Eval error: %v\n", err)
		c.JSON(500, gin.H{"error": "internal error"})
		return
	}

	// 解析返回结果
	arr, ok := res.([]interface{})
	if !ok || len(arr) < 4 {
		fmt.Printf("unexpected redis eval result: %v\n", res)
		c.JSON(500, gin.H{"error": "internal error"})
		return
	}
	// 把值转换为 int64
	succeededStr := fmt.Sprintf("%v", arr[0])
	failedStr := fmt.Sprintf("%v", arr[1])
	totalStr := fmt.Sprintf("%v", arr[2])
	changedFlagStr := fmt.Sprintf("%v", arr[3])

	succeeded, _ := strconv.ParseInt(succeededStr, 10, 64)
	failedCnt, _ := strconv.ParseInt(failedStr, 10, 64)
	total, _ := strconv.ParseInt(totalStr, 10, 64)
	changedFlag, _ := strconv.ParseInt(changedFlagStr, 10, 64)
	uintTaskID, _ := strconv.ParseUint(taskID, 10, 64)
	// 如果所有子任务完成（成功+失败 >= total），可以做后续处理
	if succeeded+failedCnt >= total && total > 0 && changedFlag == 1 {
		fmt.Printf("All I2V subtasks completed for main task %s\n", taskID)

		// SSE通知
		payload := struct {
			UserID uint64 `json:"user_id"`
			TaskID uint64 `json:"task_id"`
			Status string `json:"status"`
			Result string `json:"result,omitempty"`
		}{
			UserID: 0,
			TaskID: uintTaskID,
			Status: "falied",
			Result: "暂时不搞",
		}

		if hub := sse.GetHub(); hub != nil {
			if b, err := json.Marshal(payload); err == nil {
				hub.PublishTopic("0", b)
			}
		}
	}

	// 如果全部成功，则触发视频拼接（此处异步触发）
	// 只处理一次全部成功的情况
	if succeeded == total && total > 0 && changedFlag == 1 {
		fmt.Printf("All I2V subtasks succeeded for main task %s, starting video concatenation\n", taskID)
		go func(tid string) {
			// util.FFmpeg 目前在实现中使用硬编码的 key。建议 future 改为接受 taskID。
			util.FFmpeg(tid)
			payload := struct {
				UserID uint64 `json:"user_id"`
				TaskID uint64 `json:"task_id"`
				Status string `json:"status"`
				Result string `json:"result,omitempty"`
			}{
				UserID: 0,
				TaskID: uintTaskID,
				Status: "succeeded",
				Result: "暂时不搞",
			}

			if hub := sse.GetHub(); hub != nil {
				if b, err := json.Marshal(payload); err == nil {
					hub.PublishTopic("0", b)
				}
			}
			//通知用户
		}(taskID)
	}

	c.JSON(200, gin.H{"status": "callback received"})
}
