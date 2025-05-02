// internal/api/routes.go
package api

import "github.com/gin-gonic/gin"

// RegisterRoutes 注册所有API路由
func (s *Server) RegisterRoutes() {
	// 公开路由
	s.router.POST("/auth/token", s.handleGetToken)
	s.router.GET("/health", s.handleHealthCheck)

	// 需要认证的路由组
	auth := s.router.Group("/")
	auth.Use(AuthMiddleware())
	auth.Use(RateLimitMiddleware())
	{
		// 聊天相关
		auth.POST("/chat/completions/stream", s.handleChatCompletions)

		// 文档相关
		auth.POST("/docs/search", s.handleVectorSearch)
		auth.POST("/docs/index", s.handleIndexDocument)
		auth.GET("/docs/:id", s.handleGetDocument)

		// Wiki相关
		auth.POST("/wiki/generate", s.handleGenerateWiki)
		auth.POST("/wiki/export", s.handleExportWiki)

		// 仓库相关
		auth.POST("/repo/analyze", s.handleAnalyzeRepo)
		auth.POST("/repo/sync", s.handleSyncRepo)

		// 向量相关
		auth.POST("/vectors/search", s.handleVectorSearch)
		auth.POST("/vectors/index", s.handleIndexVectors)
		auth.DELETE("/vectors/:id", s.handleDeleteVector)
	}
}

// 获取Token
func (s *Server) handleGetToken(c *gin.Context) {
	c.JSON(200, gin.H{"token": "mock-token"})
}

// 健康检查
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// 文档向量搜索
func (s *Server) handleVectorSearch(c *gin.Context) {
	c.JSON(200, gin.H{"result": "vector search"})
}

// 文档索引
func (s *Server) handleIndexDocument(c *gin.Context) {
	c.JSON(200, gin.H{"result": "index document"})
}

// 获取文档
func (s *Server) handleGetDocument(c *gin.Context) {
	c.JSON(200, gin.H{"result": "get document"})
}

// 仓库同步
func (s *Server) handleSyncRepo(c *gin.Context) {
	c.JSON(200, gin.H{"result": "sync repo"})
}

// 向量索引
func (s *Server) handleIndexVectors(c *gin.Context) {
	c.JSON(200, gin.H{"result": "index vectors"})
}

// 删除向量
func (s *Server) handleDeleteVector(c *gin.Context) {
	c.JSON(200, gin.H{"result": "delete vector"})
}
