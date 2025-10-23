package worker

import (
	"V2V/queue"
	"V2V/store"
	"V2V/task"
	"context"
	"fmt"
	"log"

	"google.golang.org/genai"
)

type VideoProcessor struct {
	queue queue.MessageQueue
	store store.TaskStore
}

func NewVideoProcessor(q queue.MessageQueue, s store.TaskStore) *VideoProcessor {
	return &VideoProcessor{queue: q, store: s}
}

func (p *VideoProcessor) Start() {
	taskChan, err := p.queue.Consume()
	if err != nil {
		log.Fatalf("Failed to consume tasks: %v", err)
	}

	for t := range taskChan {
		go p.processTask(t)
	}
}

func (p *VideoProcessor) processTask(t task.VideoTask) {
	// 更新状态为处理中
	t.Status = "processing"
	p.store.SetTask(t)

	// 调用视频拆解API（模拟耗时操作）
	result, err := p.callVideoAnalysisAPI(t.VideoURL)

	// 更新任务状态
	if err != nil {
		t.Status = "failed"
		log.Printf("Task %s failed: %v", t.TaskID, err)
	} else {
		t.Status = "completed"
		t.Result = result
		log.Printf("Task %s completed", t.TaskID)
	}

	p.store.SetTask(t)
}

// 模拟视频分析API调用
func (p *VideoProcessor) callVideoAnalysisAPI(url string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	parts := []*genai.Part{
		genai.NewPartFromText("Please summarize the video in 3 sentences."),
		genai.NewPartFromURI(url, "video/mp4"),
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, _ := client.Models.GenerateContent(
		ctx,
		"gemini-2.5-flash",
		contents,
		nil,
	)

	fmt.Println(result.Text())
	return result.Text(), nil
}
