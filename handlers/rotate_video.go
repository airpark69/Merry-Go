package handlers

import (
	"fmt"
	strings2 "github.com/savsgio/gotils/strings"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var muxRotateVideo sync.Mutex
var err error

// 주기 변경을 위한 구조체
type ChangeInterval struct {
	Interval time.Duration
}

func RotateInteval() {
	// 초기 주기 설정 (10초)
	initialInterval := 2 * time.Second
	newLength := 0
	changeIntervalChan := make(chan ChangeInterval)
	quit := make(chan struct{})
	ticker := time.NewTicker(initialInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			newLength, err = RotateVideo(changeIntervalChan)
			if err != nil {
				log.Println(err)
			}
			ticker.Stop()
			ticker = time.NewTicker(time.Duration(newLength) * time.Second)
		case <-quit:
			log.Println("종료 시그널 받음")
			ticker.Stop()
			return
		}
	}
}

// RotateVideo rotate hls playlist from head to tail
func RotateVideo(changeIntervalChan chan<- ChangeInterval) (int, error) {
	if merryGo.IsEmpty() || merryGo.Count == 1 {
		return 10, nil
	}

	muxRotateVideo.Lock()
	defer muxRotateVideo.Unlock()

	// Read the main playlist content
	mainPlaylist, err := os.ReadFile(mainPlaylistFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
		return 10, err
	}
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

	headStart, headEnd, headLength := merryGo.Head.Rider.Info()
	lastSegNum := 0
	mainLines, lastSegNum, err = rotatePlayList(mainLines, headStart, headEnd)
	if err != nil {
		return headLength, err
	}

	headStart, headEnd, err = rotateSegment(lastSegNum, headStart, headEnd)
	if err != nil {
		// TODO -- 에러가 날 경우를 대비해서 임시 파일에서 작업 -> 모든 작업 정상적으로 동작 -> 원본 파일 수정 -> 임시 파일 삭제 의 로직을 넣어야 제대로 된 멱등성이 보장됨
		log.Println(err)
		return headLength, err
	}
	merryGo.Head.Rider.Update(headStart, headEnd)
	headStart, headEnd, headLength = merryGo.Head.Rider.Info()

	_ = merryGo.Rotate()

	headStart, headEnd, headLength = merryGo.Head.Rider.Info()

	// Update Sequence (첫번째로 읽어올 Segment 파일 번호)
	updateDurationTag(mainLines)
	updateSequenceTag(mainLines, headStart)
	// Write the combined lines back to the main playlist
	err = os.WriteFile(mainPlaylistFile, []byte(strings.Join(mainLines, "\n")), 0644)
	if err != nil {
		return headLength, err
	}

	return headLength, nil
}

/*
rotatePlayList: hls 플레이리스트 내의 순서를 rotate에 맞게 변경하는 함수

PlayListLines: 원본 플레이리스트
start: 변경할 segment 넘버 시작
end: 변경할 segment 넘버 끝

return: 변경된 플레이리스트 []string, 변경 전 getLastSegmentNum() 결과 int, 에러 error
*/
func rotatePlayList(PlayListLines []string, start int, end int) ([]string, int, error) {
	tmpLines := make([]string, len(PlayListLines))
	for i, v := range PlayListLines {
		tmpLines[i] = v
	}
	startSegLine := fmt.Sprintf(SEGNAME+"%d.ts", start)
	endSegLine := fmt.Sprintf(SEGNAME+"%d.ts", end)
	startIndex := 0
	endIndex := 0

	lastSegNum, err := getLastSegmentNum()
	if err != nil {
		return nil, 0, err
	}
	//lastSegNum := start
	beforeSegNum := lastSegNum

	if start == end {
		for i, v := range tmpLines {
			if v == startSegLine {
				startIndex = i - 1
				endIndex = i
			}
		}
	} else {
		for i, v := range tmpLines {
			if v == startSegLine {
				startIndex = i - 1

			} else if v == endSegLine {
				endIndex = i
			}
		}
	}

	for i := startIndex + 1; i <= endIndex; i += 2 {
		tmpLines[i] = fmt.Sprintf(SEGNAME+"%d.ts", lastSegNum)
		PlayListLines[i] = tmpLines[i]
		lastSegNum++
	}

	// 구분자 태그가 가장 처음에 위치한 상태로 초기화
	subSlice := []string{"#EXT-X-DISCONTINUITY"}
	// 슬라이스에서 start 인덱스부터 end 인덱스까지의 데이터 추출
	subSlice = append(subSlice, tmpLines[startIndex:endIndex+1]...) // end 인덱스는 포함되지 않으므로 endIndex + 1으로 지정

	// 슬라이스에서 해당 구간을 제거
	// PlayListLines[endIndex+2:] 인 이유는 각 세그먼트들 사이에 #EXT-X-DISCONTINUITY 가 구분자 태그로 삽입되어있기 때문
	PlayListLines = append(PlayListLines[:startIndex], PlayListLines[endIndex+2:]...)

	// 추출한 데이터를 원래 슬라이스의 맨 뒤에 붙이기
	PlayListLines = append(PlayListLines, subSlice...)

	return PlayListLines, beforeSegNum, nil
}

func rotateSegment(lastSegNum int, start int, end int) (int, int, error) {
	// 소스 디렉토리 내의 모든 파일 읽기
	files, err := os.ReadDir(absHlsDir)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read source directory: %w", err)
	}

	beforeSegs := make([]string, end-start+1)
	for i := start; i <= end; i++ {
		beforeSegs = append(beforeSegs, fmt.Sprintf(SEGNAME+"%d.ts", i))
	}
	headStart := lastSegNum
	// 세그먼트 파일 복사 -> 복사 없이 가능
	for _, file := range files {
		if strings2.Include(beforeSegs, file.Name()) {
			newFileName := fmt.Sprintf(SEGNAME+"%d.ts", lastSegNum)
			lastSegNum++
			sourceFilePath := filepath.Join(absHlsDir, file.Name())
			destFilePath := filepath.Join(absHlsDir, newFileName)
			err := os.Rename(sourceFilePath, destFilePath)
			if err != nil {
				// TODO -- 변경 중에 에러가 날 경우 트랙잭션 개념으로 전체가 취소가 되는 예외처리가 이뤄져야 함
				return 0, 0, fmt.Errorf("failed to rename file %s to %s: %w", sourceFilePath, destFilePath, err)
			}
			log.Printf("[Rotate] renamed %s to %s\n", sourceFilePath, destFilePath)
		}
	}
	headEnd := lastSegNum - 1
	return headStart, headEnd, nil
}
