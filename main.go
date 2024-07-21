package main

import (
	"Merry-Go/database"
	"Merry-Go/handlers"
	"fmt"
	"github.com/gofiber/websocket/v2"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
)

var mode = false // true -> 카메라 업로드를 통한 실시간 라이브 , false -> 파일 업로드를 통한 시청 방식
var err error

func main() {
	//tmpLines := []string{"#EXTM3U", "#EXT-X-VERSION:3", "#EXT-X-TARGETDURATION:11", "#EXT-X-MEDIA-SEQUENCE:0", "#EXTINF:10.666667,", "seg5.ts", "#EXTINF:10.666667,", "seg6.ts #EXTINF:7.366667,", "seg7.ts", "#EXTINF:6.933333,", "seg4.ts"}
	//subSlice := tmpLines[4 : 9+1]
	//
	//log.Println(subSlice)
	//
	//return
	// 초기 주기 설정 (10초)
	initialInterval := 10 * time.Second
	changeIntervalChan := make(chan handlers.ChangeInterval)
	quit := make(chan struct{})

	// 고루틴에서 주기적으로 함수 실행
	go func() {
		ticker := time.NewTicker(initialInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				err = handlers.RotateVideo(changeIntervalChan)
				if err != nil {
					log.Println(err)
				}
			case newInterval := <-changeIntervalChan:
				log.Println(newInterval.Interval.String() + " 로 주기 변경")
				ticker.Stop()
				ticker = time.NewTicker(newInterval.Interval)
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()

	// Initialize database
	database.InitDatabase()

	app := fiber.New(fiber.Config{
		BodyLimit: 10 * 1024 * 1024, // 10MB
	})
	app.Use(cors.New())
	app.Use(logger.New())

	// 환경 변수 읽기
	modeStr, exists := os.LookupEnv("MODE")
	if !exists {
		fmt.Println("The environment variable MODE is not set.")
		return
	}

	// 문자열을 Boolean으로 변환
	mode, err = strconv.ParseBool(modeStr)
	if err != nil {
		fmt.Printf("Error parsing MODE: %v\n", err)
		return
	}

	// 메세지 전달용 웹소켓 실행
	go handlers.HandleMessages()

	// 웹 소켓 핸들러 설정
	app.Get("/ws", websocket.New(handlers.HandleConnections))

	/////////////////////////////////////////////////////// 카메라에서 다이렉트로 전송 받는 경우

	if mode {
		// 서버 시작 시 Camera 업로드를 위한 ffmpeg 실행
		go handlers.StartFfmpeg()
		// 픽셀 보드 관련 소켓 연결 설정
		go handlers.HandlePixelMessages()
		app.Get("/wsp", websocket.New(handlers.HandlePixelConnections))
	} else {
		// 비디오 업로드 -> HLS 변환
		app.Post("/uploadVideo", handlers.UploadHandler)

		// TODO - RotateVideo를 주기적으로 실행 시키도록 만들고 head의 총 재생시간 만큼 주기를 잡고 반복적으로 실행시킨다
	}

	///////////////////////////////////////////////////////

	// HLS 파일이 있는 디렉토리를 설정합니다.
	app.Use("/hls", func(c *fiber.Ctx) error {
		c.Set("Access-Control-Allow-Origin", "*")
		c.Set("Cache-Control", "no-cache")
		return c.Next()
	})
	app.Static("/hls", "static/hls")

	// HTML 파일이 있는 디렉토리를 설정하고, 로그를 추가합니다.
	app.Get("/", func(c *fiber.Ctx) error {
		return handlers.FileServerHandler(c)
	})

	app.Get("/checkMode", handlers.CreateCheckModeHandler(mode))

	log.Println("Starting server on :18080")
	if err := app.Listen(":18080"); err != nil {
		log.Fatal(err)
	}

}
