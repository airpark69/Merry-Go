package handlers

import (
	"Merry-Go/data_struct"
	"errors"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"io"
	"log"
	"math"
	"mime/multipart"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var muxUploadVideo sync.Mutex

const (
	hlsDir             = "static/hls"
	tmpDir             = "upload_video_tmp"
	SEGNAME            = "seg"
	SPLITER            = "_"
	PLAYLIST           = "playlist"
	LENGTH_ADJUST      = 1.4
	TAG_TARGETDURATION = "#EXT-X-TARGETDURATION"
	TAG_MEDIALENGTH    = "#EXTINF"
)

var absHlsDir, _ = filepath.Abs(hlsDir)
var tmpHlsDir, _ = filepath.Abs(tmpDir)
var tempLines = []string{
	"#EXTM3U",
	"#EXT-X-VERSION:3",
	TAG_TARGETDURATION + ":13",
	"#EXT-X-ALLOW-CACHE:NO", // 캐시 여부
	"#EXT-X-MEDIA-SEQUENCE:0",
}
var mainPlaylistFile = filepath.Join(absHlsDir, PLAYLIST+".m3u8")
var merryGo = data_struct.NewMerryGo(10)

/* UploadHandler handles file uploads and converts them to HLS format
 */
func UploadHandler(c *fiber.Ctx) error {
	file, err := c.FormFile("video")
	if err != nil {
		log.Println("Failed to retrieve file from form-data: ", err)
		return c.Status(fiber.StatusBadRequest).SendString("Failed to retrieve file from form-data")
	}

	if merryGo.IsFull() {
		return c.Status(fiber.StatusBadRequest).SendString("Merry-Go is Full")
	}

	// Save the uploaded file to the server
	tempFilePath := filepath.Join(os.TempDir(), file.Filename)
	defer func(filePath string) {
		err := deleteTempUploadedFile(filePath)
		if err != nil {
			log.Println("Failed to delete temp uploaded file: ", err)
		}
	}(tempFilePath)

	tempFile, err := os.Create(tempFilePath)
	if err != nil {
		log.Println("Failed to create temporary file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create temporary file")
	}
	defer func(tempFile *os.File) {
		err := tempFile.Close()
		if err != nil {
			log.Println("Failed to close temporary file: ", err)
		}
	}(tempFile)

	uploadedFile, err := file.Open()
	if err != nil {
		log.Println("Failed to open uploaded file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to open uploaded file")
	}
	defer func(uploadedFile multipart.File) {
		err := uploadedFile.Close()
		if err != nil {
			log.Println("Failed to close uploaded file: ", err)
		}
	}(uploadedFile)

	if _, err := io.Copy(tempFile, uploadedFile); err != nil {
		log.Println("Failed to save file: ", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to save file")
	}

	muxUploadVideo.Lock()
	defer muxUploadVideo.Unlock()

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
	//mainPlaylistFile := filepath.Join(absHlsDir, "playlist.m3u8")
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
	segCount, err := getLastSegmentNum()
	if err != nil {
		return err
	}
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
				if strings.HasPrefix(line, TAG_MEDIALENGTH+":") {
					segCount++
				}
			}
		}
	}

	// temp segment를 복사
	filteredLines, segmentData, err := copySegments(tempPlaylistFile, tempSegmentName, absHlsDir)
	if err != nil {
		return err
	}

	// MerryGo에 Segment 데이터 삽입
	err = merryGo.Append(segmentData)
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
	// Remove the #EXT-X-ENDLIST tag - for Live Streaming
	// #EXT-X-ENDLIST tag 는 플레이 리스트가 끝나는 지점을 의미
	var mainLines []string
	for _, line := range rawMainLines {
		trimmedLine := strings.TrimSpace(line)
		if trimmedLine != "" && trimmedLine != "#EXT-X-ENDLIST" {
			mainLines = append(mainLines, line)
		}
	}

	// #EXT-X-DISCONTINUITY 태그 - 세그먼트 간 구분자 삽입
	combinedLines := append(mainLines, "#EXT-X-DISCONTINUITY")
	// Combine the main playlist and new segment lines
	combinedLines = append(combinedLines, filteredLines...)
	updateDurationTag(combinedLines)

	// Write the combined lines back to the main playlist
	err = os.WriteFile(mainPlaylistFile, []byte(strings.Join(combinedLines, "\n")), 0644)
	if err != nil {
		return err
	}

	return nil
}

/*
getLastSegmentNum 현재 segment 번호를 카운트해서 마지막 segment 파일의 숫자보다 1만큼 큰 숫자를 반환

예시: seg1.ts seg2.ts 가 있으면 3을 반환
*/
func getLastSegmentNum() (int, error) {
	// 폴더 내 파일 목록 읽기
	files, err := os.ReadDir(absHlsDir)
	if err != nil {
		log.Fatal(err)
		return 0, nil
	}

	// 정규 표현식 컴파일
	re := regexp.MustCompile(SEGNAME + `(\d+)\.ts`)

	maxNumber := 0

	// 파일 목록 순회
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// 파일 이름에서 숫자 추출
		matches := re.FindStringSubmatch(file.Name())
		if len(matches) > 1 {
			number, err := strconv.Atoi(matches[1])
			if err != nil {
				return maxNumber, err
			}
			if number > maxNumber {
				maxNumber = number
			}
		}
	}

	// 마지막 숫자보다 1 큰 값
	nextNumber := maxNumber + 1
	return nextNumber, nil
}

// deleteTempSegments 함수는 업로드 시에 넣었던 임시 폴더 내의 업로드 파일을 삭제합니다.
func deleteTempUploadedFile(filePath string) error {
	log.Printf("Deleting file: %s\n", filePath)
	err = os.Remove(filePath)
	if err != nil {
		log.Printf("Deleting file Error: %s\n", err)
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
	newTargetDuration := fmt.Sprintf("%s:%d", TAG_TARGETDURATION, int(math.Ceil(maxDuration)))
	for i, line := range combinedLines {
		if strings.HasPrefix(line, TAG_TARGETDURATION+":") {
			combinedLines[i] = newTargetDuration
			break
		}
	}
}

// updateEndListTag 인자로 받은 플레이 리스트 문자열 배열 내에 #EXT-X-ENDLIST 태그를 업데이트 합니다.
func updateEndListTag(combinedLines []string) []string {
	return append(combinedLines, "#EXT-X-ENDLIST")
}

// updateSequenceTag 인자로 받은 플레이 리스트 문자열 배열 내의 #EXT-X-MEDIA-SEQUENCE 태그를 업데이트 합니다.
func updateSequenceTag(combinedLines []string, start int) {
	newSequence := fmt.Sprintf("#EXT-X-MEDIA-SEQUENCE:%d", start)
	for i, line := range combinedLines {
		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			combinedLines[i] = newSequence
			break
		}
	}
}

// findMaxDuration 플레이 리스트 내의 segment들 중에서 가장 긴 길이를 찾습니다
func findMaxDuration(combinedLines []string) float64 {
	// Find the maximum segment duration
	maxDuration := 0.0
	for _, line := range combinedLines {
		if strings.HasPrefix(line, TAG_MEDIALENGTH+":") {
			var duration float64
			_, _ = fmt.Sscanf(line, "%s:%f,", TAG_MEDIALENGTH, &duration)
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
func copySegments(tempPlaylistPath, tempSegmentName string, destDir string) ([]string, *data_struct.Segment, error) {
	// 업로드 플레이 리스트 읽기
	var segmentLines []string
	var segmentData = &data_struct.Segment{Start: 0, End: 0, Length: 0}
	segCount, err := getLastSegmentNum()
	if err != nil {
		return segmentLines, segmentData, err
	}

	//segmentData.Name = tempSegmentName
	// TODO -- hls 플레이리스트에서 파일 간의 분할을 위한 태그가 있음. 이에 따라서 segment 이름과 숫자를 지정하지 않고 uuid 값 그대로 사용 가능, 이게 더 로직상 간단하고 직관적이라 변경하는게 좋을듯

	tempPlaylist, err := os.ReadFile(tempPlaylistPath)
	if err != nil {
		return segmentLines, segmentData, err
	}

	// 업로드 플레이 리스트 -> 문자열 배열으로 변환
	segmentLines = strings.Split(string(tempPlaylist), "\n")
	filteredLines := []string{}
	tmpCount := segCount
	segLength := 0.0
	for _, line := range segmentLines {
		// #EXTINF
		if strings.HasPrefix(line, TAG_MEDIALENGTH+":") {
			filteredLines = append(filteredLines, line)
			parts := strings.Split(line, ":")
			number, err := strconv.ParseFloat(parts[1][:len(parts[1])-1], 64)
			if err != nil {
				fmt.Println("Error:", err)
				return segmentLines, segmentData, err
			}
			segLength += number
		} else if strings.HasPrefix(line, tempSegmentName) {
			parts := strings.Split(line, SPLITER)
			if len(parts) < 2 {
				return segmentLines, segmentData, errors.New("invalid segment name")
			}
			newSegLine := fmt.Sprintf(SEGNAME+"%d.ts", tmpCount)
			// 세그먼트 부분만 사용
			filteredLines = append(filteredLines, newSegLine)
			tmpCount++
		}
	}
	segmentData.Length = int(math.Ceil(segLength * LENGTH_ADJUST))

	// .m3u8 파일의 디렉토리 추출
	sourceDir := filepath.Dir(tempPlaylistPath)

	// .m3u8 파일 이름 추출 (확장자 제거)
	baseName := strings.TrimSuffix(filepath.Base(tempPlaylistPath), filepath.Ext(tempPlaylistPath))

	// 대상 디렉토리가 존재하는지 확인하고, 없으면 생성
	if _, err := os.Stat(destDir); os.IsNotExist(err) {
		err = os.MkdirAll(destDir, os.ModePerm)
		if err != nil {
			return filteredLines, segmentData, fmt.Errorf("failed to create destination directory: %w", err)
		}
	}

	// 소스 디렉토리 내의 모든 파일 읽기
	files, err := os.ReadDir(sourceDir)
	if err != nil {
		return filteredLines, segmentData, fmt.Errorf("failed to read source directory: %w", err)
	}

	// 세그먼트 파일 복사
	segmentData.Start = segCount
	for _, file := range files {
		if strings.HasPrefix(file.Name(), baseName) && strings.HasSuffix(file.Name(), ".ts") {
			newFileName := fmt.Sprintf(SEGNAME+"%d.ts", segCount)
			segCount++
			sourceFilePath := filepath.Join(sourceDir, file.Name())
			destFilePath := filepath.Join(destDir, newFileName)
			err := copyFile(sourceFilePath, destFilePath)
			if err != nil {
				return filteredLines, segmentData, fmt.Errorf("failed to copy file %s to %s: %w", sourceFilePath, destFilePath, err)
			}
			log.Printf("Copied %s to %s\n", sourceFilePath, destFilePath)
		}
	}
	segmentData.End = segCount - 1

	return filteredLines, segmentData, nil
}

func LoadHls() error {
	// Read the main playlist content
	mainPlaylist, err := os.ReadFile(mainPlaylistFile)
	if err != nil {
		log.Println("MainPlayList가 존재하지 않습니다. 파일이 없다고 가정하고 서버를 부팅합니다.")
		return nil
	}
	log.Println("MainPlayList가 존재합니다. 기존 파일들을 Merry-Go에 입력합니다.")
	rawMainLines := strings.Split(string(mainPlaylist), "\n")

	// 정규 표현식 컴파일
	re := regexp.MustCompile(SEGNAME + `(\d+)\.ts`)

	startIndex := 0
	endIndex := 0
	segLength := 0.0

	// 파일 목록 순회
	for _, line := range rawMainLines {
		// #EXTINF
		if strings.HasPrefix(line, TAG_MEDIALENGTH+":") {
			parts := strings.Split(line, ":")
			number, err := strconv.ParseFloat(parts[1][:len(parts[1])-1], 64)
			if err != nil {
				fmt.Println("Error:", err)
				return err
			}
			segLength += number
		} else {
			// 파일 이름에서 숫자 추출
			matches := re.FindStringSubmatch(line)
			if len(matches) > 1 {
				number, err := strconv.Atoi(matches[1])
				if err != nil {
					return err
				}
				if startIndex == 0 {
					startIndex = number
					endIndex = number
				} else {
					endIndex = number
				}
			} else if line == "#EXT-X-DISCONTINUITY" {
				err = merryGo.Append(&data_struct.Segment{Start: startIndex, End: endIndex, Length: int(math.Ceil(segLength * LENGTH_ADJUST))})
				log.Printf("Merry-Go %d 번째 데이터 : %d, %d, %f", merryGo.Count, startIndex, endIndex, segLength)
				startIndex = 0
				endIndex = 0
				segLength = 0.0
				if err != nil {
					return err
				}
			}
		}
	}

	if startIndex != 0 && endIndex != 0 && segLength != 0.0 {
		err = merryGo.Append(&data_struct.Segment{Start: startIndex, End: endIndex, Length: int(math.Ceil(segLength * LENGTH_ADJUST))})
		log.Printf("Merry-Go %d 번째 데이터 : %d, %d, %f", merryGo.Count, startIndex, endIndex, segLength)
		startIndex = 0
		endIndex = 0
		segLength = 0.0
		if err != nil {
			return err
		}
	}

	return nil
}
