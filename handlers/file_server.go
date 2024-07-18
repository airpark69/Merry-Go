package handlers

import (
	"github.com/gofiber/fiber/v2"
)

// 파일 서버 요청을 처리하는 핸들러
// HTML 파일이 있는 디렉토리를 설정하고, 로그를 추가합니다.
func FileServerHandler(c *fiber.Ctx) error {

	return c.SendFile("static/html/index.html") // 적절한 파일 경로로 수정
}
