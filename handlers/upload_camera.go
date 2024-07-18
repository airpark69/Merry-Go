package handlers

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

const (
	rtpPort = "15000"
)

var (
	cmd     *exec.Cmd
	muxCmd  sync.Mutex
	sdpFile = "./stream.sdp"
)

var sdpAbsPath, _ = filepath.Abs(sdpFile)

func StartFfmpeg() {
	for {
		muxCmd.Lock()
		// SDP 파일 생성
		sdpContent := `v=0
o=- 0 0 IN IP4 0.0.0.0
s=No Name
c=IN IP4 0.0.0.0
t=0 0
a=tool:libavformat 58.76.100
m=video ` + rtpPort + ` RTP/AVP 96
b=AS:200
a=rtpmap:96 H264/90000
`
		if err := os.WriteFile(sdpAbsPath, []byte(sdpContent), 0644); err != nil {
			fmt.Printf("failed to write SDP file: %v\n", err)
		}

		// 이미 실행 중인 ffmpeg 프로세스가 있으면 종료
		if cmd != nil {
			if err := cmd.Process.Kill(); err != nil {
				fmt.Printf("failed to kill process: %v\n", err)
			}
		}

		// FFmpeg 명령어 구성
		ffmpegCmd := exec.Command("ffmpeg",
			"-protocol_whitelist", "file,udp,rtp",
			"-i", sdpAbsPath,
			"-c:v", "copy",
			"-f", "hls",
			"-hls_time", "5",
			"-hls_list_size", "5",
			"-hls_flags", "delete_segments",
			"-hls_segment_filename", absHlsDir+"/"+SEGNAME+"%05d.ts",
			absHlsDir+"/playlist.m3u8",
		)

		// // 표준 출력과 오류를 연결
		// ffmpegCmd.Stdout = log.Writer()
		// ffmpegCmd.Stderr = log.Writer()

		// ffmpeg 명령어 실행
		if err := ffmpegCmd.Start(); err != nil {
			fmt.Printf("Failed to start FFmpeg: %v\n", err)
			muxCmd.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}

		cmd = ffmpegCmd
		muxCmd.Unlock()

		// FFmpeg 프로세스가 종료될 때까지 대기
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("FFmpeg exited with error: %v\n", err)
		} else {
			fmt.Println("FFmpeg exited successfully")
		}

		// 재시작 전 짧은 대기
		time.Sleep(5 * time.Second)
	}
}
