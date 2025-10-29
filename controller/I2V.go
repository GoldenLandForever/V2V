package controller

import (
	"V2V/dao/store"
	"context"
	"fmt"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

const prompt = "### 分镜脚本\n\n**项目名称:** 极速猫咪日记\n**视频主题:** 一只快乐的猫咪驾驶摩托车飞驰在路上，享受速度与自由。\n**目标受众:** 喜爱动物、幽默感强的社交媒体用户、广告受众。\n**整体风格:** 动感、活泼、充满正能量。\n\n---\n\n**镜号: 001**\n*   **景别:** 中近景 (Medium Close-up)\n*   **画面内容:** 一只胖乎乎的橙色虎斑猫，咧嘴大笑，眼睛微闭，神情愉悦而夸张地驾驶一辆黑色复古摩托车行驶在宽阔的公路上。猫咪身体略微前倾，双手紧握车把。背景是飞速后退的绿色树林和蓝天，路面有清晰的黄线。画面带有强烈的速度感模糊。\n*   **台词 / 旁白:** (背景音乐歌词) \"Give me everything...\"\n*   **运镜方式:** 侧向跟踪镜头，低角度略微仰拍，镜头随摩托车高速平移，营造极强的速度感和动势。\n*   **音效:** 激昂的电子舞曲，摩托车引擎轰鸣声，风声呼啸。\n*   **时长:** 2.5秒\n*   **备注:** 强调猫咪享受驾驶的自由与快乐，表情夸张生动，开场即抓住观众眼球。\n*   **图片生成提示词:** A cheerful, overweight orange tabby cat with a wide-open laughing mouth and closed eyes, riding a black vintage motorcycle on a highway. Dynamic low-angle shot, strong motion blur in background, bright daylight.\n*   **视频生成提示词:** An overweight orange tabby cat, laughing with closed eyes, riding a black vintage motorcycle on a highway. The camera tracks alongside at a low angle, showing the cat moving forward dynamically with strong horizontal motion blur in the background, featuring green trees and a clear blue sky.\n\n---\n\n**镜号: 002**\n*   **景别:** 中近景 (Medium Close-up)\n*   **画面内容:** 画面中的橙色虎斑猫表情从开怀大笑迅速转变为专注凝视前方，嘴巴闭合，眼睛睁开，略显坚定与认真。它身体姿态更加前倾，尾巴高高翘起，随着摩托车高速行驶而轻微摆动。摩托车依然在公路上疾驰，背景模糊不变。\n*   **台词 / 旁白:** (背景音乐歌词) \"...it took for me.\"\n*   **运镜方式:** 侧向跟踪镜头，角度略微提升，保持与猫咪的平视视角，镜头平稳跟进，捕捉表情变化。\n*   **音效:** 激昂的电子舞曲，摩托车引擎轰鸣声，风声呼啸。\n*   **时长:** 1.5秒\n*   **备注:** 展现猫咪从极度兴奋到专注驾驶的瞬间情绪切换，增强叙事趣味性和猫咪的“专业”感。\n*   **图片生成提示词:** A focused, determined overweight orange tabby cat with open eyes and closed mouth, leaning forward aggressively, riding a black vintage motorcycle on a highway. Its tail is up, dynamic tracking shot, strong motion blur in background, bright daylight.\n*   **视频生成提示词:** An overweight orange tabby cat, initially laughing, then quickly changing to a focused, determined expression with open eyes and closed mouth, riding a black vintage motorcycle. The camera tracks alongside at eye level, showing the cat's tail swaying and strong motion blur in the background as it speeds along the highway.\n\n---\n\n**镜号: 003**\n*   **景别:** 中景 (Medium Shot)\n*   **画面内容:** 镜头逐渐拉远，展现橙色虎斑猫驾驶摩托车的整体画面，背景是广阔的蓝天白云和笔直延伸的公路。猫咪再次恢复开怀大笑的表情，享受着开阔环境带来的自由感。远处的绿色树林和路肩清晰可见，增强了空间感和环境的壮丽。\n*   **台词 / 旁白:** (背景音乐歌词) \"Give me everything...\"\n*   **运镜方式:** 跟踪拉远镜头，从略近的景别逐渐拉远，同时保持与摩托车同速平移，突出环境的开阔与猫咪的渺小却又自由。\n*   **音效:** 激昂的电子舞曲，摩托车引擎声，宽阔环境下的风声。\n*   **时长:** 2.0秒\n*   **备注:** 通过景别的变化强调环境的广阔和猫咪的自由驰骋，形成视觉上的对比，并为下一高潮做铺垫。\n*   **图片生成提示词:** An ecstatic, overweight orange tabby cat with a wide-open laughing mouth, riding a black vintage motorcycle on a vast highway. Medium shot, showing expansive blue sky with fluffy clouds, green trees, and the long road ahead. Dynamic motion blur in background, sunny daylight.\n*   **视频生成提示词:** An ecstatic overweight orange tabby cat, laughing joyfully, riding a black vintage motorcycle on a highway. The camera performs a tracking pull-out, revealing more of the vast blue sky with white clouds and the long road, maintaining strong motion blur in the background as the cat speeds forward.\n\n---\n\n**镜号: 004**\n*   **景别:** 中近景 (Medium Close-up)\n*   **画面内容:** 镜头再次快速拉近，聚焦在橙色虎斑猫和摩托车上。猫咪的笑容更加狂野和富有感染力，嘴巴张得更开，仿佛在发出胜利的咆哮或享受极致速度带来的嘶吼。画面角度略低，突出摩托车的冲击力和猫咪的霸气。背景的公路和树林再次强烈虚化，强化速度感和冲击力。\n*   **台词 / 旁白:** (背景音乐歌词) \"...I'm too fast, I'm too fast...\"\n*   **运镜方式:** 跟踪推近镜头，低角度侧向追踪，镜头小幅摇摆，增强动感和冲击力，营造高潮氛围。\n*   **音效:** 激昂的电子舞曲节奏感更强，摩托车引擎声更轰鸣，风声更凌厉。\n*   **时长:** 3.0秒\n*   **备注:** 展现高潮情绪，猫咪的表情和姿态充满力量感和征服欲，为视频结尾带来强烈的视觉和情感冲击。\n*   **图片生成提示词:** A fiercely joyful, overweight orange tabby cat with a wide-open roaring mouth, riding a black vintage motorcycle on a highway. Dynamic medium close-up, slightly low angle, emphasizing power and speed. Strong motion blur in background of road and trees, bright daylight.\n*   **视频生成提示词:** A fiercely joyful overweight orange tabby cat, with a wide-open roaring mouth, riding a black vintage motorcycle. The camera dynamically tracks and pushes in at a low angle, slightly swaying, intensifying the sense of speed and exhilaration, with strong motion blur in the background of the highway and trees.\n\n---"

func SubmitI2VTask(c *gin.Context) {
	// 请确保您已将 API Key 存储在环境变量 ARK_API_KEY 中
	// 初始化Ark客户端，从环境变量中读取您的API Key
	client := arkruntime.NewClientWithApiKey(
		// 从环境变量中获取您的 API Key。此为默认方式，您可根据需要进行修改
		os.Getenv("ARK_API_KEY"),
	)
	ctx := context.Background()
	//替换为您的 Model ID
	modelEp := "doubao-seedance-1-0-lite-i2v-250428"
	URL1 := "https://ark-content-generation-v2-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-4-0/0217617023391280522eb7202f365aca7e866281abfc7246c7d9c_0.jpeg?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20251029%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20251029T014645Z&X-Tos-Expires=86400&X-Tos-Signature=17e8eaa54b5d2ad4e8da41fa92e70ac0b0eb1395007a194417f8189d4654dba4&X-Tos-SignedHeaders=host&x-tos-process=image%2Fwatermark%2Cimage_YXNzZXRzL3dhdGVybWFyay5wbmc_eC10b3MtcHJvY2Vzcz1pbWFnZS9yZXNpemUsUF85"
	URL2 := "https://ark-content-generation-v2-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-4-0/0217617023391280522eb7202f365aca7e866281abfc7246c7d9c_1.jpeg?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20251029%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20251029T014646Z&X-Tos-Expires=86400&X-Tos-Signature=cb3de522a56962592c357ae5c20d55695ed058bc01f43d005e30940efcc59bc7&X-Tos-SignedHeaders=host&x-tos-process=image%2Fwatermark%2Cimage_YXNzZXRzL3dhdGVybWFyay5wbmc_eC10b3MtcHJvY2Vzcz1pbWFnZS9yZXNpemUsUF85"
	URL3 := "https://ark-content-generation-v2-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-4-0/0217617023391280522eb7202f365aca7e866281abfc7246c7d9c_2.jpeg?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20251029%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20251029T014647Z&X-Tos-Expires=86400&X-Tos-Signature=a314c0c454cb5930c6eeba5845e98a75579b43e29670247d829c6f4c80cce846&X-Tos-SignedHeaders=host&x-tos-process=image%2Fwatermark%2Cimage_YXNzZXRzL3dhdGVybWFyay5wbmc_eC10b3MtcHJvY2Vzcz1pbWFnZS9yZXNpemUsUF85"
	URL4 := "https://ark-content-generation-v2-cn-beijing.tos-cn-beijing.volces.com/doubao-seedream-4-0/0217617023391280522eb7202f365aca7e866281abfc7246c7d9c_3.jpeg?X-Tos-Algorithm=TOS4-HMAC-SHA256&X-Tos-Credential=AKLTYWJkZTExNjA1ZDUyNDc3YzhjNTM5OGIyNjBhNDcyOTQ%2F20251029%2Fcn-beijing%2Ftos%2Frequest&X-Tos-Date=20251029T014649Z&X-Tos-Expires=86400&X-Tos-Signature=b6f0da53b371ab0567d3f63d302b276312da048a1265f7e178d32f821bcc745e&X-Tos-SignedHeaders=host&x-tos-process=image%2Fwatermark%2Cimage_YXNzZXRzL3dhdGVybWFyay5wbmc_eC10b3MtcHJvY2Vzcz1pbWFnZS9yZXNpemUsUF85"
	fmt.Println("----- create content generation task -----")
	// 创建视频生成任务
	createReq := model.CreateContentGenerationTaskRequest{
		Model: modelEp,
		Content: []*model.CreateContentGenerationContentItem{
			{
				// 文本提示词与参数组合
				Type: model.ContentGenerationContentItemTypeText,
				Text: volcengine.String(prompt),
			},
			{
				// 第一张参考图片URL
				// 参考图需要传入1～4张
				Type: model.ContentGenerationContentItemTypeImage,
				ImageURL: &model.ImageURL{
					URL: URL1,
				},
				Role: volcengine.String("reference_image"),
			},
			{
				// 第二张参考图片URL
				Type: model.ContentGenerationContentItemTypeImage,
				ImageURL: &model.ImageURL{
					URL: URL2,
				},
				Role: volcengine.String("reference_image"),
			},
			{
				// 第三张参考图片URL
				Type: model.ContentGenerationContentItemTypeImage,
				ImageURL: &model.ImageURL{
					URL: URL3,
				},
				Role: volcengine.String("reference_image"),
			},
			{
				// 第三张参考图片URL
				Type: model.ContentGenerationContentItemTypeImage,
				ImageURL: &model.ImageURL{
					URL: URL4,
				},
				Role: volcengine.String("reference_image"),
			},
		},
	}

	createResponse, err := client.CreateContentGenerationTask(ctx, createReq)
	if err != nil {
		fmt.Printf("create content generation error: %v", err)
		return
	}
	fmt.Printf("Task Created with ID: %s", createResponse.ID)
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
