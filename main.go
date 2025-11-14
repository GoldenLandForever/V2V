package main

import (
	"V2V/controller"
	"V2V/dao/mysql"
	"V2V/dao/store"
	"V2V/middlewares"
	"V2V/pkg/queue"
	"V2V/pkg/snowflake"
	sse "V2V/pkg/sse"
	"fmt"
	"log"
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"
	"os"
	"strings"
	"time"

	_ "V2V/docs"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

func init() {
	// 如果环境变量未设置，则设置默认值
	// if os.Getenv("GEMINI_API_KEY") == "" {
	// 	// os.Setenv("GEMINI_API_KEY", "AIzaSyCXCqko6fnnjE_s2RE-oNL_rPCKvMTilbg")
	// 	os.Setenv("GEMINI_API_KEY", "AIzaSyAkQeviVNOjOVfdya8re4UvhILeTspFqqU")
	// }
	os.Setenv("GEMINI_API_KEY", "AIzaSyCXCqko6fnnjE_s2RE-oNL_rPCKvMTilbg")
	if os.Getenv("ARK_API_KEY") == "" {
		os.Setenv("ARK_API_KEY", "1b9ef66f-0934-4e09-bcd7-5ebf52808b57")
	}
}

// @title V2V API
// @version 1.0
// @description 视频处理相关 API 接口文档
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host 198.168.1.50:8080
// @BasePath /
// @schemes http https
func main() {

	if err := mysql.Init(); err != nil {
		fmt.Printf("init mysql failed, err:%v\n", err)
		return
	}
	// 初始化单例 RabbitMQ
	dsn := "amqp://admin:123456@192.168.1.50:5672/"
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

	// 初始化单例 I2V RabbitMQ
	if err := queue.InitI2VRabbitMQ(dsn); err != nil {
		log.Fatalf("Failed to init I2V RabbitMQ: %v", err)
	}
	i2vRabbitMQ, err := queue.GetI2VRabbitMQ()
	if err != nil {
		log.Fatalf("Failed to get I2V RabbitMQ instance: %v", err)
	}
	defer i2vRabbitMQ.Close()
	go func() {
		if err := i2vRabbitMQ.ConsumeI2V(); err != nil {
			log.Fatalf("I2V rabbit consume failed: %v", err)
		}
	}()
	//初始化延迟队列 I2V RabbitMQ
	err = queue.InitDelayedI2VQueue(dsn)
	if err != nil {
		log.Fatalf("Failed to init delayed I2V RabbitMQ: %v", err)
	}

	err = store.Init("192.168.1.50:6379")
	if err != nil {
		log.Fatalf("Failed to init Redis: %v", err)
	}

	//初始化雪花算法
	err = snowflake.Init(1)
	if err != nil {
		log.Fatalf("Failed to init Snowflake: %v", err)
	}

	r := gin.Default()

	//CORS 配置：如果设置了环境变量 CORS_ALLOWED_ORIGINS（逗号分隔），则使用白名单；否则使用宽松的默认策略（方便开发）
	allowed := os.Getenv("CORS_ALLOWED_ORIGINS")
	if allowed == "" {
		r.Use(cors.New(cors.Config{
			AllowOrigins:     []string{"*"}, // 开发环境允许所有来源
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS", "PATCH"},
			AllowHeaders:     []string{"*"}, // 允许所有头，包括 Authorization
			ExposeHeaders:    []string{"Content-Length", "Content-Range", "Authorization"},
			AllowCredentials: false, // 当 AllowOrigins 为 * 时，此项必须为 false
			MaxAge:           12 * time.Hour,
		}))
	} else {
		origins := strings.Split(allowed, ",")
		r.Use(cors.New(cors.Config{
			AllowOrigins:     origins,
			AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
			ExposeHeaders:    []string{"Content-Length", "Content-Range"},
			AllowCredentials: true,
			MaxAge:           12 * time.Hour,
		}))
	}

	// 初始化并启动 SSE hub
	sseHub := sse.NewHub()
	sse.SetDefaultHub(sseHub)
	go sseHub.Run()
	registerPprof(r)
	r.GET("/events", sse.ServeSSE)

	// 静态视频目录（返回 /videos/<taskid>.mp4）
	r.Static("/videos", "./public/videos")
	r.Static("/pic", "./public/pic")

	// 公开路由（无需登录）

	// Swagger 文档路由
	//  /home/xc/go/lib/bin/swag init -g main.go -o ./docs
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// 受保护的 API（需要 JWT）
	v1 := r.Group("/api/v1")
	v1.POST("/login", controller.LoginHandler)               // 登陆业务
	v1.POST("/signup", controller.SignUpHandler)             // 注册业务
	v1.GET("/refresh_token", controller.RefreshTokenHandler) // 刷新accessToken
	v1.Use(middlewares.JWTAuthMiddleware())
	{
		v1.POST("/V2T", controller.SubmitV2TTask)
		v1.POST("/V2T/LoraText", controller.LoraText)
		v1.POST("/T2I", controller.SubmitT2ITask)
		v1.GET("/V2T/:task_id", controller.GetV2TTaskResult)
		v1.POST("/I2V", controller.SubmitI2VTask)
		v1.GET("/I2V/:task_id", controller.GetI2VTaskResult)
		v1.POST("/I2VCallback/:task_id", controller.I2VCallback)
		v1.GET("/FFmpeg/:task_id", controller.FFmpegHandler)
	}

	r.Run(":8080")

}

func registerPprof(router *gin.Engine) {
	// pprof 路由组
	pprofGroup := router.Group("/debug/pprof")
	{
		pprofGroup.GET("/", pprofHandler(pprof.Index))
		pprofGroup.GET("/cmdline", pprofHandler(pprof.Cmdline))
		pprofGroup.GET("/profile", pprofHandler(pprof.Profile))
		pprofGroup.POST("/symbol", pprofHandler(pprof.Symbol))
		pprofGroup.GET("/symbol", pprofHandler(pprof.Symbol))
		pprofGroup.GET("/trace", pprofHandler(pprof.Trace))
		pprofGroup.GET("/allocs", pprofHandler(pprof.Handler("allocs").ServeHTTP))
		pprofGroup.GET("/block", pprofHandler(pprof.Handler("block").ServeHTTP))
		pprofGroup.GET("/goroutine", pprofHandler(pprof.Handler("goroutine").ServeHTTP))
		pprofGroup.GET("/heap", pprofHandler(pprof.Handler("heap").ServeHTTP))
		pprofGroup.GET("/mutex", pprofHandler(pprof.Handler("mutex").ServeHTTP))
		pprofGroup.GET("/threadcreate", pprofHandler(pprof.Handler("threadcreate").ServeHTTP))
	}
}

// pprofHandler 将 http.HandlerFunc 转换为 gin.HandlerFunc
func pprofHandler(handler http.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		handler.ServeHTTP(c.Writer, c.Request)
	}
}
