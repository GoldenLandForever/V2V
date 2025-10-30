package main

import (
	"V2V/controller"
	"V2V/dao/store"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	sse "V2V/pkg/sse"
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

	// 初始化单例 T2I RabbitMQ
	if err := queue.InitT2IRabbitMQ(dsn); err != nil {
		log.Fatalf("Failed to init T2I RabbitMQ: %v", err)
	}
	t2iRabbitMQ, err := queue.GetT2IRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to get T2I RabbitMQ instance: %v", err)
	}
	defer t2iRabbitMQ.Close()
	go func() {
		if err := t2iRabbitMQ.ConsumeT2I(); err != nil {
			log.Fatalf("T2I rabbit consume failed: %v", err)
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

	// 初始化并启动 SSE hub
	sseHub := sse.NewHub()
	sse.SetDefaultHub(sseHub)
	go sseHub.Run()

	r.GET("/events", sse.ServeSSE)

	r.POST("/V2T", controller.SubmitV2TTask)
	r.POST("/V2T/LoraText", controller.LoraText)
	r.POST("/T2I", controller.SubmitT2ITask)
	r.GET("/V2T/:task_id", controller.GetV2TTaskResult)
	r.POST("/I2V", controller.SubmitI2VTask)
	r.GET("/I2V/:task_id", controller.GetI2VTaskResult)
	r.Run()
}
