package controller

import (
	"context"
	"fmt"
	"os"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
	"github.com/volcengine/volcengine-go-sdk/volcengine"
)

func SubmitI2VTask() {
	client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
	ctx := context.Background()

	req := model.CreateContentGenerationTaskRequest{
		Model: "doubao-seedance-1-0-pro-fast-251015",
		Content: []*model.CreateContentGenerationContentItem{
			&model.CreateContentGenerationContentItem{
				Type: "text",
				Text: volcengine.String("女孩抱着狐狸，女孩睁开眼，温柔地看向镜头，狐狸友善地抱着，镜头缓缓拉出，女孩的头发被风吹动  --ratio adaptive  --dur 5"),
			},
			&model.CreateContentGenerationContentItem{
				Type: "image_url",
				ImageURL: &model.ImageURL{
					URL: "https://ark-project.tos-cn-beijing.volces.com/doc_image/i2v_foxrgirl.png",
				},
			},
		},
	}

	resp, err := client.CreateContentGenerationTask(ctx, req)
	if err != nil {
		fmt.Printf("create content generation error: %v\n", err)
		return
	}
	fmt.Printf("Task Created with ID: %s\n", resp.ID)
}
