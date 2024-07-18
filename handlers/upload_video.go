package handlers

import (
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

var muxUploadVideo sync.Mutex

const (
	hlsDir  = "static/hls"
	tmpDir  = "upload_video_tmp"
	SEGNAME = "seg"
	SPLITER = "_"
)

var absHlsDir, _ = filepath.Abs(hlsDir)
var tmpHlsDir, _ = filepath.Abs(tmpDir)
var segCount = 0
var tempLines = []string{
	"#EXTM3U",
	"#EXT-X-VERSION:3",
	"#EXT-X-TARGETDURATION:13",
	"#EXT-X-MEDIA-SEQUENCE:0",
}

// UploadHandler handles file uploads and converts them to HLS format
func UploadHandler(c *fiber.Ctx) error {
	file, err := c.FormFile("video")
	if err != nil {
		log.Println("Failed to retrieve file from form-data: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Failed to retrieve file from form-data")
	}

	// Save the uploaded file to the server
	tempFilePath := filepath.Join(os.TempDir(), file.Filename)
	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		log.Println("Failed to create temporary file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create temporary file")
	}
	defer tempFile.Close()

	uploadedFile, err := file.Open()
	if err != nil {
		log.Println("Failed to open uploaded file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open uploaded file")
	}
	defer uploadedFile.Close()

	if _, err := io.Copy(tempFile, uploadedFile); err != nil {
		log.Println("Failed to save file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save file")
	}

	muxUploadVideo.Lock()
	defer muxUploadVideo.Unlock()

	// Convert the uploaded file to HLS format
	//absHlsDir, err := filepath.Abs(hlsDir)
	//if err != nil {
	//	log.Println("Failed to get absolute path of HLS directory: ", err)
	//	return c.Status(fiber.StatusInternalServerError).SendString("Failed to get absolute path of HLS directory")
	//}

	// tmpHlsDir 디렉토리가 존재하는지 확인하고, 없으면 생성
	if _, err := os.Stat(tmpHlsDir); os.IsNotExist(err) {
		err = os.MkdirAll(tmpHlsDir, os.ModePerm)
		if err != nil {
			log.Fatal("failed to create directory: %w", err)
		}
	}

	// UUID 생성
	fileKey := uuid.New().String()
	tempSegmentName := fileKey + SPLITER + SEGNAME
	tempPlaylistFilePath := filepath.Join(tmpHlsDir, tempSegmentName+".m3u8")
	err = convertToHLS(tempFilePath, tempPlaylistFilePath)
	if err != nil {
		log.Println("Failed to convert video to HLS format: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to convert video to HLS format")
	}

	defer func(directory string, pattern string) {
		err := deleteTempSegments(directory, pattern)
		if err != nil {
			log.Println("Failed to delete TempSegments", err)
		}
	}(tmpHlsDir, fileKey)

	// Append new segments to the main playlist
	mainPlaylistFile := filepath.Join(absHlsDir, "playlist.m3u8")
	err = appendToPlaylist(mainPlaylistFile, tempPlaylistFilePath, tempSegmentName)
	if err != nil {
		log.Println("Failed to update HLS playlist: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to update HLS playlist")
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// convertToHLS converts a video file to HLS format
func convertToHLS(inputFilePath, outputFilePath string) error {
	cmd := exec.Command("ffmpeg", "-i", inputFilePath, "-c:v", "copy", "-c:a", "copy",
		"-start_number", "0", "-hls_time", "10", "-hls_list_size", "0", "-f", "hls", outputFilePath)

	// Capture stderr output
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	// Read stderr output
	slurp, _ := io.ReadAll(stderr)
	if err := cmd.Wait(); err != nil {
		log.Println("ffmpeg command failed: ", err)
		log.Println("ffmpeg stderr: ", string(slurp))
		return err
	}

	return nil
}

// appendToPlaylist appends segments from the new file to the existing playlist
func appendToPlaylist(mainPlaylistFile, tempPlaylistFile string, tempSegmentName string) error {
	if segCount == 0 {
		mainPlaylist, err := os.ReadFile(mainPlaylistFile)
		if err != nil {
			// mainPlaylist가 존재하지 않을 경우 그대로 진행
			if os.IsNotExist(err) {
				log.Println("mainPlaylist does not exist but ok: ", err)
			} else {
				log.Fatalf("Error reading file: %v", err)
				return err
			}
		} else {
			rawMainLines := strings.Split(string(mainPlaylist), "\n")
			// Remove the #EXT-X-ENDLIST tag
			// #EXT-X-ENDLIST tag 는 플레이 리스트가 끝나는 지점을 의미

			for _, line := range rawMainLines {
				if strings.HasPrefix(line, "#EXTINF:") {
					segCount++
				}
			}
		}
	}

	// temp segment를 복사
	filteredLines, err := copySegments(tempPlaylistFile, tempSegmentName, absHlsDir)
	if err != nil {
		return err
	}

	// Read the main playlist content
	mainPlaylist, err := os.ReadFile(mainPlaylistFile)
	if err != nil {
		if os.IsNotExist(err) {
			// mainPlaylist가 존재하지 않을 경우 새로 생성
			log.Printf("mainPlaylist does not exist -- creating: %s\n", mainPlaylistFile)
			combinedLines := append(tempLines, filteredLines...)
			combinedLines = append(combinedLines, "#EXT-X-ENDLIST")
			updateDurationTag(combinedLines)
			err = os.WriteFile(mainPlaylistFile, []byte(strings.Join(combinedLines, "\n")), 0644)
			if err != nil {
				return err
			}
			return nil
		} else {
			log.Fatalf("Error reading file: %v", err)
			return err
		}
	}

	// 기존 플레이 리스트 -> mainLines 문자열 배열으로 변환
	rawMainLines := strings.Split(string(mainPlaylist), "\n")
	// Remove the #EXT-X-ENDLIST tag
	// #EXT-X-ENDLIST tag 는 플레이 리스트가 끝나는 지점을 의미
	var mainLines []string
	for _, line := range rawMainLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" && trimmedLine != "#EXT-X-ENDLIST" {
			mainLines = append(mainLines, line)
		}
	}

	// Combine the main playlist and new segment lines
	combinedLines := append(mainLines, filteredLines...)
	updateDurationTag(combinedLines)

	// Add the #EXT-X-ENDLIST tag back to the combined lines
	combinedLines = append(combinedLines, "#EXT-X-ENDLIST")

	// Write the combined lines back to the main playlist
	err = os.WriteFile(mainPlaylistFile, []byte(strings.Join(combinedLines, "\n")), 0644)
	if err != nil {
		return err
	}

	return nil
}

// deleteTempSegments 함수는 주어진 디렉토리 내에서 pattern 에 해당하는 문자열으로 시작하는 파일을 모두 삭제합니다.
func deleteTempSegments(directory string, pattern string) error {
	err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 파일 이름이 "temp_segment"로 시작하는지 확인
		if !info.IsDir() && strings.HasPrefix(filepath.Base(path), pattern) {
			log.Printf("Deleting file: %s\n", path)
			err = os.Remove(path)
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

// updateDurationTag 인자로 받은 플레이 리스트 문자열 배열 내의 #EXT-X-TARGETDURATION 태그를 업데이트 합니다.
func updateDurationTag(combinedLines []string) {
	maxDuration := findMaxDuration(combinedLines)
	newTargetDuration := fmt.Sprintf("#EXT-X-TARGETDURATION:%d", int(math.Ceil(maxDuration)))
	for i, line := range combinedLines {
		if strings.HasPrefix(line, "#EXT-X-TARGETDURATION:") {
			combinedLines[i] = newTargetDuration
			break
		}
	}
}

// findMaxDuration 플레이 리스트 내의 segment들 중에서 가장 긴 길이를 찾습니다
func findMaxDuration(combinedLines []string) float64 {
	// Find the maximum segment duration
	maxDuration := 0.0
	for _, line := range combinedLines {
		if strings.HasPrefix(line, "#EXTINF:") {
			var duration float64
			_, _ = fmt.Sscanf(line, "#EXTINF:%f,", &duration)
			if duration > maxDuration {
				maxDuration = duration
			}
		}
	}

	return maxDuration
}

// copyFile 함수는 src 파일을 dst 파일로 복사합니다.
func copyFile(src, dst string) error {
	// 원본 파일 열기
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	// 원본 파일의 파일 정보 가져오기
	sourceFileInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	// 목적지 파일 생성
	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	// 파일 복사
	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	// 복사된 파일의 권한 설정
	err = destinationFile.Chmod(sourceFileInfo.Mode())
	if err != nil {
		return err
	}

	return nil
}

// copySegments 함수는 .m3u8 파일과 같은 폴더에 있는 세그먼트 파일을 특정 폴더로 복사합니다.
// 복사할 때 segment들은 seg%d.ts 의 형태로 segCount에 따라서 다르게 복사됩니다.
func copySegments(tempPlaylistPath, tempSegmentName string, destDir string) ([]string, error) {
	// 업로드 플레이 리스트 읽기
	var segmentLines []string
	tempPlaylist, err := os.ReadFile(tempPlaylistPath)
	if err != nil {
		return segmentLines, err
	}

	// 업로드 플레이 리스트 -> 문자열 배열으로 변환
	segmentLines = strings.Split(string(tempPlaylist), "\n")
	filteredLines := []string{}
	tmpCount := segCount
	for _, line := range segmentLines {
		// #EXTINF
		if strings.HasPrefix(line, "#EXTINF:") {
			filteredLines = append(filteredLines, line)
		} else if strings.HasPrefix(line, tempSegmentName) {
			parts := strings.Split(line, SPLITER)
			if len(parts) < 2 {
				return segmentLines, errors.New("invalid segment name")
			}
			newSegLine := fmt.Sprintf(SEGNAME+"%d.ts", tmpCount)
			// 세그먼트 부분만 사용
			filteredLines = append(filteredLines, newSegLine)
			tmpCount++
		}
	}

	// .m3u8 파일의 디렉토리 추출
	sourceDir := filepath.Dir(tempPlaylistPath)

	// .m3u8 파일 이름 추출 (확장자 제거)
	baseName := strings.TrimSuffix(filepath.Base(tempPlaylistPath), filepath.Ext(tempPlaylistPath))

	// 대상 디렉토리가 존재하는지 확인하고, 없으면 생성
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		err = os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return filteredLines, fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// 소스 디렉토리 내의 모든 파일 읽기
	files, err := os.ReadDir(sourceDir)
	if err != nil {
		return filteredLines, fmt.Errorf("failed to read source directory: %w", err)
	}

	// 세그먼트 파일 복사
	for _, file := range files {
		if strings.HasPrefix(file.Name(), baseName) && strings.HasSuffix(file.Name(), ".ts") {
			newFileName := fmt.Sprintf(SEGNAME+"%d.ts", segCount)
			segCount++
			sourceFilePath := filepath.Join(sourceDir, file.Name())
			destFilePath := filepath.Join(destDir, newFileName)
			err := copyFile(sourceFilePath, destFilePath)
			if err != nil {
				return filteredLines, fmt.Errorf("failed to copy file %s to %s: %w", sourceFilePath, destFilePath, err)
			}
			log.Printf("Copied %s to %s\n", sourceFilePath, destFilePath)
		}
	}

	return filteredLines, nil
}
