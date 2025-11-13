package util

import (
	"fmt"
	"io"
	"net/http"
	"os"
)

func DownloadImages(imageURL, task_id string, index int) error {
	// 创建输出文件
	filename := fmt.Sprintf(task_id+"_%d.jpg", index)
	filepath := "./public/pic/" + filename
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 发送HTTP请求
	resp, err := http.Get(imageURL)
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 将响应体写入文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}
