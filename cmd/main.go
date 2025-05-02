// cmd/server/main.go
package main

import (
	"log"

	"github.com/deepwiki-go/internal/api"
	"github.com/deepwiki-go/internal/config"
	"github.com/joho/godotenv"
)

func main() {
	// 加载环境变量 (Load environment variables)
	if err := godotenv.Load(); err != nil {
		log.Println("警告: 未找到 .env 文件，将使用环境变量")
	}

	// 初始化配置 (Initialize configuration from YAML/env)
	cfg, err := config.LoadConfig("") // Use default path "internal/config/config.yaml"
	if err != nil {
		log.Fatalf("加载配置失败 (Failed to load configuration): %v", err)
	}

	// API Key Check (Optional - moved from direct env check)
	// You might want to add checks here or let downstream components handle missing keys
	if cfg.Google.APIKey == "" {
		log.Println("警告: 未找到 Google API 密钥 (Google API Key not found). Google RAG 功能可能无法工作。")
	}
	if cfg.OpenAIAPIKey == "" {
		log.Println("警告: 未找到 OpenAI API 密钥 (OpenAI API Key not found). OpenAI RAG 功能可能无法工作。")
	}

	// 创建并启动服务器 (Create and start the server)
	server := api.NewServer(cfg)
	log.Printf("启动流式 API 服务，端口 %s (Starting streaming API service on port %s)\n", cfg.Server.Port, cfg.Server.Port)
	if err := server.Start(); err != nil {
		log.Fatalf("服务器错误 (Server error): %v", err)
	}
}
