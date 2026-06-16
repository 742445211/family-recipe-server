//go:build ignore

// 直接将识别结果写入 scan（运维/联调）。用法：
//   go run ./apply_fridge_result.go <scan_id> '<json_detail>'
package main

import (
	"encoding/json"
	"log"
	"os"
	"strconv"

	"recipe-server/config"
	"recipe-server/internal/service"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatal("usage: apply_fridge_result <scan_id> '<json_detail>'")
	}
	scanID, _ := strconv.ParseUint(os.Args[1], 10, 64)
	if err := config.Load("config.yaml"); err != nil {
		log.Fatal(err)
	}
	db, err := gorm.Open(mysql.Open(config.AppConfig.MySQL.DSN()), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	svc := service.NewFridgeService(db, nil)
	if err := svc.ApplyRecognizeResult(scanID, json.RawMessage(os.Args[2])); err != nil {
		log.Fatal(err)
	}
	log.Printf("scan %d -> done", scanID)
}
