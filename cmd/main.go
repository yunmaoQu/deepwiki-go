// cmd/server/main.go
package main

import (
        "log"
        "os"
        
        "github.com/joho/godotenv"        
        "deepwiki-go/internal/api"
        "deepwiki-go/internal/config"
)

func main() {
        // 加载环境变量
        if err := godotenv.Load(); err != nil {
                log.Println("警告: 未找到 .env 文件，将使用环境变量")
        }
        
        // 初始化配置
        cfg := config.NewConfig()
        
        // 检查必需的环境变量
        if cfg.GoogleAPIKey == "" || cfg.OpenAIAPIKey == "" {
                log.Println("警告: 缺少 API 密钥。某些功能可能无法正常工作。")
        }
        
        // 从环境变量获取端口或使用默认值
        port := cfg.Port
        if port == "" {
                port = "8001"
        }
        
        // 创建并启动服务器
        server := api.NewServer(cfg)
        log.Printf("启动流式 API 服务，端口 %s\n", port)
        if err := server.Start(); err != nil {
                log.Fatalf("服务器错误: %v", err)
        }
}