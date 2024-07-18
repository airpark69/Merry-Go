package handlers

import (
	"SnackCam/database"
	"SnackCam/models"
	"log"

	"github.com/gofiber/websocket/v2"
)

// 클라이언트와의 연결을 추적
var clients = make(map[*websocket.Conn]bool)

// 메시지 브로드캐스트 채널
var broadcast = make(chan Message)

// Message struct to hold the message data
type Message struct {
	Username string `json:"username"`
	Message  string `json:"message"`
}

// 초기 ws 연결 시 클라이언트와 소통하는 부분
// 웹소켓 연결 핸들러
func HandleConnections(c *websocket.Conn) {
	defer func(c *websocket.Conn) {
		err := c.Close()
		if err != nil {
			log.Printf("error: %v", err)
		}
	}(c)

	var msgs []models.Message
	result := database.DB.Order("created_at desc").Limit(50).Find(&msgs) // 최근 50개만 읽어옴,
	if result.Error != nil {
		log.Printf("50 Message Read error: %v", result.Error)
	}

	// 처음 접속 시 최근 50개 메세지 전송
	// api 메세지 모델로 변환 후 하나로 저장
	apiMessages := make([]Message, len(msgs))
	for i, msgModel := range msgs {
		apiMessages[len(msgs)-1-i] = Message{ // 앞에서부터 읽어서 위로 올려보내는 구조라서 가장 최근의 메세지는 가장 마지막에 들어가야함
			Username: msgModel.UserName,
			Message:  msgModel.Message,
		}
	}
	// 한 번에 전송
	if err := c.WriteJSON(apiMessages); err != nil {
		log.Printf("error: %v", err)
	}
	clients[c] = true

	for {
		var msg Message
		err := c.ReadJSON(&msg)
		if err != nil {
			log.Printf("error: %v", err)
			delete(clients, c)
			break
		}
		broadcast <- msg
	}
}

// 연결 이후 클라이언트와 소통하는 부분
func HandleMessages() {
	for {
		msg := <-broadcast

		// 메세지 DB에 저장
		createMessage(msg)

		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("error: %v", err)
				_ = client.Close()
				delete(clients, client)
			}
		}
	}
}

func createMessage(msg Message) {
	message := new(models.Message)
	message.UserName = msg.Username
	message.Message = msg.Message

	database.DB.Create(&message)
}
