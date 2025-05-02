// internal/api/handlers.go
package api

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/internal/data"
	"github.com/deepwiki-go/internal/models"
	"github.com/deepwiki-go/internal/rag"
	"github.com/gin-gonic/gin"
)

// Server 表示API服务器
type Server struct {
	router  *gin.Engine
	config  *config.Config
	manager *rag.RAGManager
}

// NewServer 创建一个新的服务器实例
func NewServer(cfg *config.Config) *Server {
	router := gin.Default()

	// 设置中间件
	router.Use(LoggingMiddleware())
	router.Use(CORSMiddleware())
	router.Use(ErrorHandlerMiddleware())

	// 初始化 RAG 管理器
	manager := rag.NewRAGManager(cfg)

	// 注册默认的 Google RAG 提供者
	googleRAG := rag.NewGoogleRAG(cfg)
	if err := manager.RegisterProvider(googleRAG); err != nil {
		// 记录错误但继续运行，因为我们可能还有其他提供者
		fmt.Printf("注册 Google RAG 提供者失败: %v\n", err)
	}

	s := &Server{
		router:  router,
		config:  cfg,
		manager: manager,
	}

	// 注册路由
	s.setupRoutes()

	return s
}

// setupRoutes 注册API路由
func (s *Server) setupRoutes() {
	// 根端点
	s.router.GET("/", s.handleRoot)

	// 聊天完成端点
	s.router.POST("/chat/completions/stream", s.handleChatCompletions)

	// Wiki生成端点
	s.router.POST("/wiki/generate", s.handleGenerateWiki)

	// Wiki导出端点
	s.router.POST("/wiki/export", s.handleExportWiki)

	// 仓库分析端点
	s.router.POST("/repo/analyze", s.handleAnalyzeRepo)

	// 获取JWT令牌端点
	s.router.POST("/token", s.handleGetToken)

	// 健康检查端点
	s.router.GET("/health", s.handleHealthCheck)

	// 向量搜索端点
	s.router.POST("/vector/search", s.handleVectorSearch)

	// 文档索引端点
	s.router.POST("/document/index", s.handleIndexDocument)

	// 获取单个文档端点
	s.router.GET("/document/:id", s.handleGetDocument)

	// 仓库同步端点
	s.router.POST("/repo/sync", s.handleSyncRepo)

	// 向量索引端点
	s.router.POST("/vector/index", s.handleIndexVectors)

	// 删除向量端点
	s.router.DELETE("/vector/:id", s.handleDeleteVector)
}

// Start 启动服务器
func (s *Server) Start() error {
	addr := ":" + s.config.Server.Port
	log.Printf("Server starting on %s", addr)
	return s.router.Run(addr)
}

// handleRoot 处理根路径
func (s *Server) handleRoot(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"version": "1.0.0",
		"name":    "DeepWiki-Go API",
	})
}

// handleChatCompletions 处理聊天完成请求
func (s *Server) handleChatCompletions(c *gin.Context) {
	var req models.ChatCompletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的请求: %v", err)})
		return
	}

	// 获取当前活动的 RAG 提供者
	provider, err := s.manager.GetActiveProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取 RAG 提供者失败: %v", err)})
		return
	}

	// 选择访问令牌 (GitHub 或 GitLab)
	accessToken := req.GitHubToken
	if accessToken == "" {
		accessToken = req.GitLabToken
	}

	// 准备仓库
	if req.RepoURL != "" {
		if err := provider.PrepareRetriever(req.RepoURL, accessToken); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("准备仓库失败: %v", err)})
			return
		}
	}

	// 获取最后一条用户消息
	var userPrompt string
	if len(req.Messages) > 0 {
		lastMsg := req.Messages[len(req.Messages)-1]
		if lastMsg.Role == "user" {
			userPrompt = lastMsg.Content
		}
	}

	if userPrompt == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未找到用户消息"})
		return
	}

	// 设置内容类型为SSE
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	// 创建一个SSE流
	clientGone := c.Stream(func(w io.Writer) bool {
		// 获取AI回复
		responseCh, err := provider.GenerateStreamingResponse(userPrompt)
		if err != nil {
			c.SSEvent("error", gin.H{"error": fmt.Sprintf("生成失败: %v", err)})
			return false
		}

		// 流式传输响应
		for chunk := range responseCh {
			c.SSEvent("message", chunk)
		}

		// 发送完成事件
		c.SSEvent("done", nil)
		return false
	})

	if !clientGone {
		// 客户端断开连接
		c.AbortWithStatus(http.StatusOK)
	}
}

// handleGenerateWiki 处理Wiki生成请求
func (s *Server) handleGenerateWiki(c *gin.Context) {
	var req struct {
		RepoURL     string `json:"repo_url"`
		GitHubToken string `json:"github_token,omitempty"`
		GitLabToken string `json:"gitlab_token,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的请求: %v", err)})
		return
	}

	// 获取当前活动的 RAG 提供者
	provider, err := s.manager.GetActiveProvider()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("获取 RAG 提供者失败: %v", err)})
		return
	}

	// 选择访问令牌
	accessToken := req.GitHubToken
	if accessToken == "" {
		accessToken = req.GitLabToken
	}

	// 初始化库管理器
	repoManager := data.NewRepositoryManager(s.config)

	// 克隆仓库
	repoPath, err := repoManager.CloneRepository(req.RepoURL, accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("克隆仓库失败: %v", err)})
		return
	}

	// 分析仓库结构
	analysis, err := repoManager.AnalyzeRepository(repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("分析仓库失败: %v", err)})
		return
	}

	// 准备RAG检索器
	if err := provider.PrepareRetriever(repoPath, accessToken); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("准备检索器失败: %v", err)})
		return
	}

	// 生成Wiki页面
	pages, err := s.generateWikiPages(analysis, req.RepoURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("生成Wiki失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pages": pages,
	})
}

// handleExportWiki 处理Wiki导出请求
func (s *Server) handleExportWiki(c *gin.Context) {
	var req models.WikiExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的请求: %v", err)})
		return
	}

	if len(req.Pages) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "未提供页面内容"})
		return
	}

	// 根据不同格式导出
	var content string
	var contentType string
	var filename string

	repoName := getRepoNameFromURL(req.RepoURL)

	switch strings.ToLower(req.Format) {
	case "markdown", "md":
		content = exportToMarkdown(req.Pages)
		contentType = "text/markdown"
		filename = fmt.Sprintf("%s-wiki.md", repoName)
	case "json":
		jsonData, err := json.MarshalIndent(req.Pages, "", "  ")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("序列化JSON失败: %v", err)})
			return
		}
		content = string(jsonData)
		contentType = "application/json"
		filename = fmt.Sprintf("%s-wiki.json", repoName)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的导出格式"})
		return
	}

	// 设置响应头
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Data(http.StatusOK, contentType, []byte(content))
}

// handleAnalyzeRepo 处理仓库分析请求
func (s *Server) handleAnalyzeRepo(c *gin.Context) {
	var req struct {
		RepoURL     string `json:"repo_url"`
		GitHubToken string `json:"github_token,omitempty"`
		GitLabToken string `json:"gitlab_token,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效的请求: %v", err)})
		return
	}

	// 选择访问令牌
	accessToken := req.GitHubToken
	if accessToken == "" {
		accessToken = req.GitLabToken
	}

	// 初始化库管理器
	repoManager := data.NewRepositoryManager(s.config)

	// 克隆仓库
	repoPath, err := repoManager.CloneRepository(req.RepoURL, accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("克隆仓库失败: %v", err)})
		return
	}

	// 分析仓库结构
	analysis, err := repoManager.AnalyzeRepository(repoPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("分析仓库失败: %v", err)})
		return
	}

	// 生成结构图
	diagram, err := generateRepoStructureDiagram(analysis)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("生成结构图失败: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"analysis": gin.H{
			"repo":    analysis,
			"diagram": diagram,
		},
	})
}

// 辅助函数

// generateWikiPages 生成Wiki页面
func (s *Server) generateWikiPages(analysis map[string]interface{}, repoURL string) ([]models.WikiPage, error) {
	// 获取当前活动的 RAG 提供者
	provider, err := s.manager.GetActiveProvider()
	if err != nil {
		return nil, fmt.Errorf("获取 RAG 提供者失败: %v", err)
	}

	// 创建页面集合
	var pages []models.WikiPage

	// 创建概述页面
	overviewPage, err := s.generateOverviewPage(analysis, repoURL, provider)
	if err != nil {
		return nil, err
	}
	pages = append(pages, overviewPage)

	// 创建架构页面
	architecturePage, err := s.generateArchitecturePage(analysis, repoURL, provider)
	if err != nil {
		return nil, err
	}
	pages = append(pages, architecturePage)

	// 为每个主要目录创建模块页面
	for dirName, dirContent := range analysis {
		if content, ok := dirContent.(map[string]interface{}); ok {
			if isNonEssentialDir(dirName) {
				continue
			}

			modulePage, err := s.generateModulePage(dirName, content, repoURL, provider)
			if err != nil {
				continue // 跳过出错的模块
			}
			pages = append(pages, modulePage)
		}
	}

	return pages, nil
}

// generateOverviewPage 生成项目概述页面
func (s *Server) generateOverviewPage(analysis map[string]interface{}, repoURL string, provider rag.RAGProvider) (models.WikiPage, error) {
	// 准备查询获取项目概述
	query := fmt.Sprintf("生成以下代码仓库的概述: %s\n\n请包括以下内容:\n- 项目名称和简短描述\n- 主要功能\n- 技术栈概览\n- 开发者指南", repoURL)

	// 检索相关文档
	docs, err := provider.RetrieveDocuments(query)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 构建上下文
	var context string
	for _, doc := range docs {
		context += doc.Text + "\n\n"
	}

	// 生成概述内容
	prompt := fmt.Sprintf("基于以下代码库的信息，生成一个项目概述页面：\n\n%s\n\n请使用Markdown格式，包括以下部分：\n1. 项目简介\n2. 主要功能\n3. 技术栈\n4. 入门指南", context)

	responseCh, err := provider.GenerateStreamingResponse(prompt)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 收集响应
	var content strings.Builder
	for chunk := range responseCh {
		content.WriteString(chunk)
	}

	// 获取仓库名称
	repoName := getRepoNameFromURL(repoURL)

	// 创建页面
	return models.WikiPage{
		ID:           "overview",
		Title:        fmt.Sprintf("%s - 项目概述", repoName),
		Content:      content.String(),
		FilePaths:    []string{"README.md", "CONTRIBUTING.md", "docs/"},
		Importance:   "high",
		RelatedPages: []string{},
	}, nil
}

// generateArchitecturePage 生成架构页面
func (s *Server) generateArchitecturePage(analysis map[string]interface{}, repoURL string, provider rag.RAGProvider) (models.WikiPage, error) {
	// 生成结构图
	diagram, err := generateRepoStructureDiagram(analysis)
	if err != nil {
		return models.WikiPage{}, err
	}

	query := "描述此代码库的整体架构、主要组件和它们之间的关系"

	// 检索相关文档
	docs, err := provider.RetrieveDocuments(query)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 构建上下文
	var context string
	for _, doc := range docs {
		context += doc.Text + "\n\n"
	}

	// 生成架构内容
	prompt := fmt.Sprintf("基于以下代码信息，生成一个架构文档：\n\n%s\n\n请使用Markdown格式，解释主要组件及其交互方式。以下是项目结构图，请在解释中引用它：\n\n```mermaid\n%s\n```", context, diagram)

	responseCh, err := provider.GenerateStreamingResponse(prompt)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 收集响应
	var content strings.Builder
	for chunk := range responseCh {
		content.WriteString(chunk)
	}

	// 获取仓库名称
	repoName := getRepoNameFromURL(repoURL)

	// 创建页面
	return models.WikiPage{
		ID:           "architecture",
		Title:        fmt.Sprintf("%s - 系统架构", repoName),
		Content:      content.String(),
		FilePaths:    []string{},
		Importance:   "high",
		RelatedPages: []string{"overview"},
	}, nil
}

// generateModulePage 生成模块页面
func (s *Server) generateModulePage(moduleName string, moduleContent interface{}, repoURL string, provider rag.RAGProvider) (models.WikiPage, error) {
	// 准备查询
	query := fmt.Sprintf("描述%s目录中的代码功能和主要组件", moduleName)

	// 检索相关文档
	docs, err := provider.RetrieveDocuments(query)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 构建上下文
	var context string
	for _, doc := range docs {
		context += doc.Text + "\n\n"
	}

	// 生成模块内容
	prompt := fmt.Sprintf("请基于以下代码信息，详细描述'%s'模块的功能、组件和用法：\n\n%s\n\n请使用Markdown格式，包括：\n1. 模块概述\n2. 主要组件和类\n3. 关键功能\n4. 与其他模块的交互\n5. 示例用法（如果适用）", moduleName, context)

	responseCh, err := provider.GenerateStreamingResponse(prompt)
	if err != nil {
		return models.WikiPage{}, err
	}

	// 收集响应
	var content strings.Builder
	for chunk := range responseCh {
		content.WriteString(chunk)
	}

	// 创建页面
	return models.WikiPage{
		ID:           fmt.Sprintf("module-%s", strings.ToLower(moduleName)),
		Title:        fmt.Sprintf("%s 模块", moduleName),
		Content:      content.String(),
		FilePaths:    []string{moduleName + "/"},
		Importance:   "medium",
		RelatedPages: []string{"architecture", "overview"},
	}, nil
}

// 生成仓库结构图
func generateRepoStructureDiagram(analysis map[string]interface{}) (string, error) {
	if structure, ok := analysis["structure"].(map[string]interface{}); ok {
		// 创建Mermaid图表
		var diagram strings.Builder
		diagram.WriteString("graph TD\n")
		diagram.WriteString("    Root[项目根目录] --> ")

		// 添加顶级目录
		var dirs []string
		for dir := range structure {
			if !strings.HasPrefix(dir, ".") && !isNonEssentialDir(dir) {
				dirs = append(dirs, dir)
			}
		}

		// 按字母顺序排序目录
		sort.Strings(dirs)

		// 添加目录节点
		for i, dir := range dirs {
			dirID := fmt.Sprintf("Dir%d", i)
			diagram.WriteString(fmt.Sprintf("%s[%s]\n", dirID, dir))

			// 递归添加目录结构
			if dirContent, ok := structure[dir].(map[string]interface{}); ok {
				addDirStructure(&diagram, dirID, dirContent, 0)
			}

			// 如果不是最后一个目录，添加Root到下一个目录的连接
			if i < len(dirs)-1 {
				diagram.WriteString("    Root --> ")
			}
		}

		// 添加样式
		diagram.WriteString("\n    classDef root fill:#f9f,stroke:#333,stroke-width:2px;\n")
		diagram.WriteString("    classDef dir fill:#bbf,stroke:#33c,stroke-width:1px;\n")
		diagram.WriteString("    classDef file fill:#bfb,stroke:#3c3,stroke-width:1px;\n")
		diagram.WriteString("    class Root root;\n")

		// 为所有目录添加dir类
		for i := range dirs {
			diagram.WriteString(fmt.Sprintf("    class Dir%d dir;\n", i))
		}

		return diagram.String(), nil
	}

	return "", fmt.Errorf("无法获取仓库结构信息")
}

// 递归添加目录结构到图表
func addDirStructure(diagram *strings.Builder, parentID string, content map[string]interface{}, depth int) {
	if depth > 2 { // 限制深度
		return
	}

	// 排序目录和文件
	var items []string
	for name := range content {
		items = append(items, name)
	}
	sort.Strings(items)

	// 最多显示5个子项
	maxItems := 5
	if len(items) > maxItems {
		items = items[:maxItems]
	}

	for i, name := range items {
		itemID := fmt.Sprintf("%s_%d", parentID, i)

		// 检查是子目录还是文件
		if subDir, ok := content[name].(map[string]interface{}); ok {
			// 这是一个目录
			diagram.WriteString(fmt.Sprintf("    %s --> %s[%s/]\n", parentID, itemID, name))
			diagram.WriteString(fmt.Sprintf("    class %s dir;\n", itemID))

			// 递归处理子目录
			addDirStructure(diagram, itemID, subDir, depth+1)
		} else {
			// 这是一个文件
			diagram.WriteString(fmt.Sprintf("    %s --> %s[%s]\n", parentID, itemID, name))
			diagram.WriteString(fmt.Sprintf("    class %s file;\n", itemID))
		}
	}

	// 如果有更多项，添加省略号
	if len(content) > maxItems {
		moreID := fmt.Sprintf("%s_more", parentID)
		diagram.WriteString(fmt.Sprintf("    %s --> %s[...更多项]\n", parentID, moreID))
	}
}

// 辅助函数

// getRepoNameFromURL 从URL中提取仓库名称
func getRepoNameFromURL(url string) string {
	// 移除协议前缀
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// 移除.git后缀
	url = strings.TrimSuffix(url, ".git")

	// 分割路径部分
	parts := strings.Split(url, "/")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}

	return "wiki"
}

// exportToMarkdown 将Wiki页面导出为Markdown
func exportToMarkdown(pages []models.WikiPage) string {
	var result strings.Builder

	// 添加标题
	result.WriteString("# DeepWiki 导出\n\n")
	result.WriteString("## 目录\n\n")

	// 生成目录
	for _, page := range pages {
		result.WriteString(fmt.Sprintf("- [%s](#%s)\n", page.Title, strings.ToLower(strings.ReplaceAll(page.ID, " ", "-"))))
	}

	result.WriteString("\n---\n\n")

	// 添加每个页面的内容
	for _, page := range pages {
		result.WriteString(fmt.Sprintf("## %s\n\n", page.Title))
		result.WriteString(page.Content)
		result.WriteString("\n\n---\n\n")
	}

	// 添加页脚
	result.WriteString("*由 DeepWiki-Go 生成*\n")

	return result.String()
}

// isNonEssentialDir 检查是否为非关键目录
func isNonEssentialDir(dirName string) bool {
	nonEssentialDirs := map[string]bool{
		"node_modules": true,
		"vendor":       true,
		"dist":         true,
		"build":        true,
		"target":       true,
		"bin":          true,
		"obj":          true,
	}

	return nonEssentialDirs[dirName]
}

// min 返回两个整数中较小的一个
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// handleGetToken 处理获取JWT令牌的请求
func (s *Server) handleGetToken(c *gin.Context) {
	// Placeholder - replace with actual token generation logic
	c.JSON(200, gin.H{"token": "mock-token"})
}

// handleHealthCheck 处理健康检查请求
func (s *Server) handleHealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{"status": "ok"})
}

// handleVectorSearch 处理向量搜索请求
func (s *Server) handleVectorSearch(c *gin.Context) {
	// Placeholder - replace with actual vector search logic
	c.JSON(200, gin.H{"result": "vector search"})
}

// handleIndexDocument 处理文档索引请求
func (s *Server) handleIndexDocument(c *gin.Context) {
	// Placeholder - replace with actual document indexing logic
	c.JSON(200, gin.H{"result": "index document"})
}

// handleGetDocument 处理获取单个文档请求
func (s *Server) handleGetDocument(c *gin.Context) {
	// Placeholder - replace with actual document retrieval logic
	c.JSON(200, gin.H{"result": "get document"})
}

// handleSyncRepo 处理仓库同步请求
func (s *Server) handleSyncRepo(c *gin.Context) {
	// Placeholder - replace with actual repo sync logic
	c.JSON(200, gin.H{"result": "sync repo"})
}

// handleIndexVectors 处理向量索引请求
func (s *Server) handleIndexVectors(c *gin.Context) {
	// Placeholder - replace with actual vector indexing logic
	c.JSON(200, gin.H{"result": "index vectors"})
}

// handleDeleteVector 处理删除向量请求
func (s *Server) handleDeleteVector(c *gin.Context) {
	// Placeholder - replace with actual vector deletion logic
	c.JSON(200, gin.H{"result": "delete vector"})
}
