package handlers

import "github.com/gofiber/fiber/v2"

// createCheckModeHandler는 mode 값을 캡처하는 클로저를 생성
func CreateCheckModeHandler(mode bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"mode": mode,
		})
	}
}
