package models

import "time"

type Pixel struct {
	Id        string `gorm:"primaryKey"`
	Color     string `gorm:"size:7"` // 색상 값은 HEX 코드 (#RRGGBB)로 저장
	CreatedAt time.Time
	UpdatedAt time.Time
}
