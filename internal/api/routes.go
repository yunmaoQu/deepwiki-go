// internal/api/router.go
package api

import (
        "github.com/gin-gonic/gin"
        "github.com/deepwiki-go/internal/config"
)

// Server 表示 API 服务器
type Server struct {
        router *gin.Engine
        config *config.Config
}

// NewServer 创建一个新的服务器实例
func NewServer(cfg *config.Config) *Server {
        router := gin.Default()
        
        // 设置 CORS
        router.Use(func(c *gin.Context) {
                c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
                c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
                c.Writer.Header().Set("Access-Control-Allow-Headers", "*")
                c.Writer.Header().Set("Access-Control-Allow-Methods", "*")
                
                if c.Request.Method == "OPTIONS" {
                        c.AbortWithStatus(204)
                        return
                }
                
                c.Next()
        })
        
        // 初始化处理器
        s := &Server{
                router: router,
                config: cfg,
        }
        
        // 注册路由
        s.registerRoutes()
        
        return s
}

// registerRoutes 注册 API 路由
func (s *Server) registerRoutes() {
        // 根端点
        s.router.GET("/", s.handleRoot)
        
        // 聊天完成端点
        s.router.POST("/chat/completions/stream", s.handleChatCompletions)
        
        // Wiki 导出端点
        s.router.POST("/export/wiki", s.handleExportWiki)
}

// Start 启动服务器
func (s *Server) Start() error {
        port := s.config.Port
        if port == "" {
                port = "8001"
        }
        return s.router.Run(":" + port)
}