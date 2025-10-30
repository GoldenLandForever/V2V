package controller

import (
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"V2V/task"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
)

func SubmitT2ITask(c *gin.Context) {
	var T2IRequest task.T2IRequest
	if err := c.ShouldBindJSON(&T2IRequest); err != nil {
		c.JSON(400, gin.H{"error": "invalid request"})
		return
	}
	taskID, err := snowflake.GetID()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to generate task ID"})
		return
	}
	var T2ITask task.T2ITask
	T2ITask.TaskID = taskID
	T2ITask.UserID = T2IRequest.UserID
	T2ITask.Prompt = T2IRequest.Text
	T2ITask.Status = task.T2IStatusPending
	T2ITask.CreatedAt = time.Now().Unix()
	T2IRequest.TaskID = taskID

	rabbitMQ, err := queue.GetT2IRabbitMQ()
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to get T2I message queue"})
		return
	}
	b, err := json.Marshal(T2ITask)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to serialize T2I task"})
		return
	}
	err = rabbitMQ.PublishT2ITask(b, T2IRequest.Priority)
	if err != nil {
		c.JSON(500, gin.H{"error": "failed to publish T2I task"})
		return
	}
	c.JSON(202, gin.H{"status": "task submitted"})

}

// func T2ITask(T2IRequest task.T2IRequest) {
// 	client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
// 	ctx := context.Background()

// 	var sequentialImageGeneration model.SequentialImageGeneration = "auto"
// 	maxImages := 10
// 	generateReq := model.GenerateImagesRequest{
// 		Model:                     "doubao-seedream-4-0-250828",
// 		Prompt:                    T2IRequest.Text,
// 		Size:                      volcengine.String("2K"),
// 		ResponseFormat:            volcengine.String(model.GenerateImagesResponseFormatURL),
// 		Watermark:                 volcengine.Bool(true),
// 		SequentialImageGeneration: &sequentialImageGeneration,
// 		SequentialImageGenerationOptions: &model.SequentialImageGenerationOptions{
// 			MaxImages: &maxImages,
// 		},
// 	}
// 	//计算执行时间
// 	starttime := time.Now()
// 	defer func() {
// 		elapsed := time.Since(starttime)
// 		fmt.Printf("T2I API call took %s\n", elapsed)
// 	}()
// 	resp, err := client.GenerateImages(ctx, generateReq)
// 	if err != nil {
// 		fmt.Printf("call GenerateImages error: %v\n", err)
// 		return
// 	}
// 	if resp.Error != nil {
// 		fmt.Printf("API returned error: %s - %s\n", resp.Error.Code, resp.Error.Message)
// 		return
// 	}
// 	// 输出生成的图片信息
// 	fmt.Printf("Generated %d images:\n", len(resp.Data))
// 	for i, image := range resp.Data {
// 		var url string
// 		if image.Url != nil {
// 			url = *image.Url
// 		} else {
// 			url = "N/A"
// 		}
// 		//储存到redis
// 		store.T2IImage(T2IRequest, url)
// 		fmt.Printf("Image %d: Size: %s, URL: %s\n", i+1, image.Size, url)
// 	}
// }
