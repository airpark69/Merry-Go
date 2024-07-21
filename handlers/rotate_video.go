package handlers

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

var muxRotateVideo sync.Mutex

// 주기 변경을 위한 구조체
type ChangeInterval struct {
	Interval time.Duration
}

// RotateVideo rotate hls playlist from head to tail
func RotateVideo(changeIntervalChan chan<- ChangeInterval) error {
	if merryGo.IsEmpty() || merryGo.Count == 1 {
		return nil
	}

	muxRotateVideo.Lock()
	defer muxRotateVideo.Unlock()

	// Read the main playlist content
	mainPlaylist, err := os.ReadFile(mainPlaylistFile)
	if err != nil {
		log.Fatalf("Error reading file: %v", err)
		return err
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

	mainLines, err = rotatePlayList(mainLines, headStart, headEnd)
	if err != nil {
		return err
	}
	_ = merryGo.Rotate()

	// Add the #EXT-X-ENDLIST tag back to the combined lines
	mainLines = append(mainLines, "#EXT-X-ENDLIST")

	headStart, headEnd, headLength = merryGo.Head.Rider.Info()

	// Update Sequence (첫번째로 읽어올 Segment 파일 번호)
	updateSequenceTag(mainLines, headStart)
	// Write the combined lines back to the main playlist
	err = os.WriteFile(mainPlaylistFile, []byte(strings.Join(mainLines, "\n")), 0644)
	if err != nil {
		return err
	}

	// TODO-- seg.ts 파일들 Rotate에 따라서 HLS 플레이리스트와 SYNC 작업

	// 주기 변경 채널에 다음 파일 길이 송신
	changeIntervalChan <- ChangeInterval{Interval: time.Duration(headLength) * time.Second}
	return nil
}

func rotatePlayList(PlayListLines []string, start int, end int) ([]string, error) {
	tmpLines := make([]string, len(PlayListLines))
	for i, v := range PlayListLines {
		tmpLines[i] = v
	}
	startSegLine := fmt.Sprintf(SEGNAME+"%d.ts", start)
	endSegLine := fmt.Sprintf(SEGNAME+"%d.ts", end)
	startIndex := 0
	endIndex := 0

	segCount, err := countSegment()
	if err != nil {
		return nil, err
	}

	for i, v := range tmpLines {
		if v == startSegLine {
			startIndex = i - 1
		} else if v == endSegLine {
			endIndex = i
		}
	}

	for i := startIndex + 1; i <= endIndex; i += 2 {
		tmpLines[i] = fmt.Sprintf(SEGNAME+"%d.ts", segCount)
		PlayListLines[i] = tmpLines[i]
		segCount++
	}

	// 슬라이스에서 start 인덱스부터 end 인덱스까지의 데이터 추출
	subSlice := tmpLines[startIndex : endIndex+1] // end 인덱스는 포함되지 않으므로 endIndex + 1으로 지정

	// 슬라이스에서 해당 구간을 제거
	PlayListLines = append(PlayListLines[:startIndex], PlayListLines[endIndex+1:]...)

	// 추출한 데이터를 원래 슬라이스의 맨 뒤에 붙이기
	PlayListLines = append(PlayListLines, subSlice...)

	return PlayListLines, nil
}
