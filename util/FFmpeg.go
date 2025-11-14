package util

import (
	"V2V/dao/store"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/volcengine/volcengine-go-sdk/service/arkruntime"
	"github.com/volcengine/volcengine-go-sdk/service/arkruntime/model"
)

// VideoProcessor 视频处理器结构体
type VideoProcessor struct {
	tempDir    string
	outputPath string
}

// NewVideoProcessor 创建新的视频处理器
func NewVideoProcessor(outputPath string) (*VideoProcessor, error) {
	tempDir, err := os.MkdirTemp("", "video_processor")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %v", err)
	}

	return &VideoProcessor{
		tempDir:    tempDir,
		outputPath: outputPath,
	}, nil
}

// Cleanup 清理临时文件
func (vp *VideoProcessor) Cleanup() {
	os.RemoveAll(vp.tempDir)
}

// DownloadVideo 下载单个视频
func (vp *VideoProcessor) DownloadVideo(url string, filename string) error {
	// 创建输出文件
	filepath := filepath.Join(vp.tempDir, filename)
	out, err := os.Create(filepath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer out.Close()

	// 发送HTTP请求
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("下载请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("下载失败，状态码: %d", resp.StatusCode)
	}

	// 复制数据到文件
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("写入文件失败: %v", err)
	}

	return nil
}

// DownloadAllVideos 并发下载所有视频
func (vp *VideoProcessor) DownloadAllVideos(urls []string) ([]string, error) {
	var wg sync.WaitGroup
	errors := make(chan error, len(urls))
	downloadedFiles := make([]string, len(urls))

	for i, url := range urls {
		wg.Add(1)
		go func(index int, videoURL string) {
			defer wg.Done()

			filename := fmt.Sprintf("video_%d%s", index, vp.getFileExtension(videoURL))
			log.Printf("正在下载: %s -> %s", videoURL, filename)

			err := vp.DownloadVideo(videoURL, filename)
			if err != nil {
				errors <- fmt.Errorf("下载视频 %d 失败: %v", index, err)
				return
			}

			downloadedFiles[index] = filename
			log.Printf("下载完成: %s", filename)
		}(i, url)
	}

	wg.Wait()
	close(errors)

	// 检查是否有错误
	var errorList []string
	for err := range errors {
		errorList = append(errorList, err.Error())
	}

	if len(errorList) > 0 {
		return nil, fmt.Errorf("下载过程中发生错误: %s", strings.Join(errorList, "; "))
	}

	return downloadedFiles, nil
}

// getFileExtension 从URL获取文件扩展名
func (vp *VideoProcessor) getFileExtension(url string) string {
	// 尝试从URL中提取扩展名
	if strings.Contains(url, ".mp4") {
		return ".mp4"
	} else if strings.Contains(url, ".avi") {
		return ".avi"
	} else if strings.Contains(url, ".mov") {
		return ".mov"
	} else if strings.Contains(url, ".mkv") {
		return ".mkv"
	} else if strings.Contains(url, ".webm") {
		return ".webm"
	}
	// 默认使用mp4
	return ".mp4"
}

// CreateConcatList 创建FFmpeg拼接列表文件
func (vp *VideoProcessor) CreateConcatList(files []string) (string, error) {
	listFile := filepath.Join(vp.tempDir, "concat_list.txt")
	file, err := os.Create(listFile)
	if err != nil {
		return "", fmt.Errorf("创建列表文件失败: %v", err)
	}
	defer file.Close()

	for _, filename := range files {
		fullPath := filepath.Join(vp.tempDir, filename)
		_, err := file.WriteString(fmt.Sprintf("file '%s'\n", fullPath))
		if err != nil {
			return "", fmt.Errorf("写入列表文件失败: %v", err)
		}
	}

	return listFile, nil
}

// ConcatVideos 使用FFmpeg拼接视频
func (vp *VideoProcessor) ConcatVideos(listFile string) error {
	// 检查FFmpeg是否可用
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return fmt.Errorf("ffmpeg未找到，请先安装ffmpeg并添加到PATH: %v", err)
	}

	audioPath := "/media/xc/my/V2V/util/backgroundmusic.mp3"

	// 检查音频文件是否存在
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		log.Printf("音频文件不存在，将生成无声视频: %s", audioPath)
		// 如果没有音频文件，使用无声视频处理
		return err
	}

	// 直接使用重新编码方式并添加音频（确保一定有声音）
	cmd := exec.Command("ffmpeg",
		"-f", "concat",
		"-safe", "0",
		"-i", listFile,
		"-i", audioPath,
		"-vf", "setpts=0.6667*PTS",
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-c:a", "aac",
		"-b:a", "128k",
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-shortest",
		vp.outputPath)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	log.Printf("执行带音频的重新编码命令: %s", cmd.String())
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("视频拼接失败: %v", err)
	}
	return nil
}

// DownloadAndConcatVideos 主函数：下载并拼接视频
func DownloadAndConcatVideos(urls []string, outputPath string) error {
	// 创建视频处理器
	processor, err := NewVideoProcessor(outputPath)
	if err != nil {
		return err
	}
	defer processor.Cleanup()

	log.Printf("开始处理 %d 个视频", len(urls))

	// 1. 下载所有视频
	downloadedFiles, err := processor.DownloadAllVideos(urls)
	if err != nil {
		return err
	}

	// 2. 创建拼接列表
	listFile, err := processor.CreateConcatList(downloadedFiles)
	if err != nil {
		return err
	}

	// 3. 拼接视频
	err = processor.ConcatVideos(listFile)
	if err != nil {
		return err
	}

	log.Printf("视频拼接完成: %s", outputPath)
	return nil
}

func FFmpeg(userId uint64, taskid string) string {
	// 使用示例
	redisclient := store.GetRedis()
	// 测试效果
	// 将存储在redis中的Zset中的视频链接对应的任务ID取出来
	keys := "user:" + strconv.FormatUint(userId, 10) + ":i2vtask:" + taskid
	val, err := redisclient.ZRange(keys, 0, -1).Result()
	if err != nil {
		log.Fatalf("无法从Redis获取任务链接: %v", err)
	}
	urls := make([]string, 0)
	for _, v := range val {
		url, err := GetVideoURL(v)
		if err != nil {
			log.Fatalf("获取视频链接失败: %v", err)
		}
		urls = append(urls, url)
	}
	// 确保输出目录存在（public/videos）
	outDir := "./public/videos"
	if err := os.MkdirAll(outDir, 0755); err != nil {
		log.Fatalf("无法创建输出目录: %v", err)
	}

	// 临时拼接输出文件（随后会合并音频生成最终文件）
	concatPath := filepath.Join(outDir, taskid+"_concat.mp4")
	finalPath := filepath.Join(outDir, taskid+".mp4")

	err = DownloadAndConcatVideos(urls, concatPath)
	if err != nil {
		log.Fatalf("处理失败: %v", err)
	}
	// 合并音频到最终输出
	err = mergeVideoAudio(concatPath, "./util/backgroundmusic.mp3", finalPath)
	if err != nil {
		log.Fatalf("合并音频失败: %v", err)
	}
	// 删除临时拼接文件
	if err := os.Remove(concatPath); err != nil {
		log.Printf("删除临时文件失败: %v", err)
	}
	log.Printf("处理完成，输出文件: %s", finalPath)
	return finalPath
}

func mergeVideoAudio(videoPath, audioPath, outputPath string) error {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath, // 输入视频文件
		"-i", audioPath, // 输入音频文件
		"-c", "copy", // 直接流拷贝，不重新编码
		"-shortest", // 以较短的流为准
		"-y",        // 覆盖输出文件
		outputPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("ffmpeg执行失败: %v, 输出: %s", err, string(output))
	}

	return nil
}

func GetVideoURL(taskID string) (string, error) {
	client := arkruntime.NewClientWithApiKey(os.Getenv("ARK_API_KEY"))
	ctx := context.Background()

	req := model.GetContentGenerationTaskRequest{}
	req.ID = taskID

	resp, err := client.GetContentGenerationTask(ctx, req)
	if err != nil {
		fmt.Printf("get content generation task error: %v\n", err)
		return "", err
	}
	store.I2VTaskVideoURL(taskID, resp.Content.VideoURL)
	return resp.Content.VideoURL, nil
}
