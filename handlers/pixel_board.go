package handlers

import (
	"SnackCam/database"
	"SnackCam/models"
	"log"

	"github.com/gofiber/websocket/v2"
	"gorm.io/gorm"
)

// 클라이언트와의 연결을 추적
var pixelClients = make(map[*websocket.Conn]bool)

// 메시지 브로드캐스트 채널
var pixelBroadcast = make(chan PixelMsg)

// Message struct to hold the message data
type PixelMsg struct {
	Id    string `json:"id"`
	Color string `json:"color"`
}

// 초기 ws 연결 시 클라이언트와 소통하는 부분
// 웹소켓 연결 핸들러
func HandlePixelConnections(c *websocket.Conn) {
	defer func(c *websocket.Conn) {
		err := c.Close()
		if err != nil {
			log.Printf("error: %v", err)
		}
	}(c)

	var pixels []models.Pixel
	result := database.DB.Find(&pixels)
	if result.Error != nil {
		log.Printf("All Pixel Read error: %v", result.Error)
	}

	// 연결된 클라이언트에 메시지 히스토리 전송
	for _, pixel := range pixels {
		msg := new(PixelMsg)
		msg.Id = pixel.Id
		msg.Color = pixel.Color

		if err := c.WriteJSON(msg); err != nil {
			log.Printf("error: %v", err)
		}
	}
	pixelClients[c] = true

	for {
		var msg PixelMsg
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(pixelClients, c)
			break
		}
		pixelBroadcast <- msg
	}
}

// 연결 이후 클라이언트와 소통하는 부분
func HandlePixelMessages() {
	for {
		msg := <-pixelBroadcast

		// 픽셀 DB에 저장
		upsertPixel(msg)

		for client := range pixelClients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				_ = client.Close()
				delete(pixelClients, client)
			}
		}
	}
}

func upsertPixel(msg PixelMsg) {
	pixel := new(models.Pixel)
	log.Println(msg)
	pixel.Id = msg.Id
	pixel.Color = msg.Color

	var existingPixel models.Pixel
	result := database.DB.First(&existingPixel, "id = ?", pixel.Id)
	if result.Error == gorm.ErrRecordNotFound {
		database.DB.Create(&pixel)
	} else {
		existingPixel.Color = pixel.Color
		database.DB.Save(&existingPixel)
	}
}
