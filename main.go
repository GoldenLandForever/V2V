package main

import (
	"V2V/api"
	"V2V/queue"
	"V2V/store"
	"V2V/worker"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func init() {
	// 如果环境变量未设置，则设置默认值
	if os.Getenv("GEMINI_API_KEY") == "" {
		os.Setenv("GEMINI_API_KEY", "AIzaSyCXCqko6fnnjE_s2RE-oNL_rPCKvMTilbg")
	}
}

func main() {
	// This is a placeholder for the main function.
	rabbitMQ, err := queue.NewRabbitMQ("amqp://admin:123456@localhost:5672/")
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer rabbitMQ.Close()

	redisStore := store.NewRedisStore("localhost:6379")

	// 启动任务处理器
	processor := worker.NewVideoProcessor(rabbitMQ, redisStore)
	go processor.Start()
	r := gin.Default()
	handler := api.NewHandler(rabbitMQ, redisStore)
	r.POST("/V2T", handler.SubmitVideoTask)
	r.GET("/V2T/:task_id", handler.GetTaskResult)
	r.Run()
}
