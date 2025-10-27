package main

import (
	"V2V/controller"
	"V2V/dao/store"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	"log"
	"os"

	"github.com/gin-gonic/gin"
)

func init() {
	// 如果环境变量未设置，则设置默认值
	if os.Getenv("GEMINI_API_KEY") == "" {
		os.Setenv("GEMINI_API_KEY", "AIzaSyCXCqko6fnnjE_s2RE-oNL_rPCKvMTilbg")
	}

	if os.Getenv("ARK_API_KEY") == "" {
		os.Setenv("ARK_API_KEY", "1b9ef66f-0934-4e09-bcd7-5ebf52808b57")
	}
}

func main() {
	// 初始化单例 RabbitMQ
	dsn := "amqp://admin:123456@localhost:5672/"
	if err := queue.InitRabbitMQ(dsn); err != nil {
		log.Fatalf("Failed to init RabbitMQ: %v", err)
	}
	rabbitMQ, err := queue.GetRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to get RabbitMQ instance: %v", err)
	}
	defer rabbitMQ.Close()
	go func() {
		if err := rabbitMQ.Consume(); err != nil {
			log.Fatalf("rabbit consume failed: %v", err)
		}
	}()

	err = store.Init("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}

	//初始化雪花算法
	err = snowflake.Init(1)
	if err != nil {
		log.Fatalf("Failed to init Snowflake: %v", err)
	}

	r := gin.Default()

	r.POST("/V2T", controller.SubmitV2TTask)
	r.POST("/V2T/LoraText", controller.LoraText)
	r.POST("/T2I", controller.SubmitT2ITask)
	r.GET("/V2T/:task_id", controller.GetV2TTaskResult)
	r.Run()
}
