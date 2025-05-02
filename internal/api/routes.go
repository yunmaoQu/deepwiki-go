// internal/api/routes.go
package api

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

// NOTE: All handler implementations (handleGetToken, handleHealthCheck, etc.) have been moved to handlers.go
