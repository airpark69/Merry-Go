package database

import (
	"SnackCam/models"
	"log"
	"os"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

var DB *gorm.DB

func InitDatabase() {
	// 데이터 디렉토리가 없으면 생성
	if _, err := os.Stat("data"); os.IsNotExist(err) {
		os.Mkdir("data", 0755)
	}

	var err error
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/db.db"
	}

	DB, err = gorm.Open(sqlite.Open(dbPath), &gorm.Config{})
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	// 데이터베이스 마이그레이션 (테이블 생성)
	DB.AutoMigrate(&models.Message{}, &models.Pixel{})
}
