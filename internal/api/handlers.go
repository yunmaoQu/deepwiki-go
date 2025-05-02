// internal/api/handlers.go
package api

import (
        "fmt"
        "net/http"
        "strings"
        "time"
        
        "github.com/gin-gonic/gin"
        "github.com/deepwiki-go/internal/data"
        "github.com/deepwiki-go/internal/models"
        "github.com/deepwiki-go/internal/rag"
)

// 处理根端点
func (s *Server) handleRoot(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
                "message": "欢迎使用流式 API",
                "version": "1.0.0",
                "endpoints": gin.H{
                        "Chat": []string{
                                "POST /chat/completions/stream - 流式聊天完成",
                        },
                        "Wiki": []string{
                                "POST /export/wiki - 以 Markdown 或 JSON 格式导出 wiki 内容",
                        },
                },
        })
}

// 处理聊天完成
func (s *Server) handleChatCompletions(c *gin.Context) {
        var request models.ChatCompletionRequest
        
        if err := c.ShouldBindJSON(&request); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效请求: %v", err)})
                return
        }
        
        // 验证请求
        if request.RepoURL == "" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "仓库 URL 是必需的"})
                return
        }
        
        if len(request.Messages) == 0 {
                c.JSON(http.StatusBadRequest, gin.H{"error": "未提供消息"})
                return
        }
        
        // 获取最后一条用户消息
        lastMessage := request.Messages[len(request.Messages)-1]
        if lastMessage.Role != "user" {
                c.JSON(http.StatusBadRequest, gin.H{"error": "最后一条消息必须来自用户"})
                return
        }
        
        // 创建一个新的 RAG 实例
        ragService := rag.NewRAG(s.config)
        
        // 确定使用哪个访问令牌
        var accessToken string
        if strings.Contains(request.RepoURL, "github.com") && request.GitHubToken != "" {
                accessToken = request.GitHubToken
        } else if strings.Contains(request.RepoURL, "gitlab.com") && request.GitLabToken != "" {
                accessToken = request.GitLabToken
        }
        
        // 准备检索器
        err := ragService.PrepareRetriever(request.RepoURL, accessToken)
        if err != nil {
                c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("准备检索器时出错: %v", err)})
                return
        }
        
        // 设置流式响应
        c.Header("Content-Type", "text/event-stream")
        c.Header("Cache-Control", "no-cache")
        c.Header("Connection", "keep-alive")
        c.Header("Transfer-Encoding", "chunked")
        
        // 处理先前的消息以构建对话历史
        for i := 0; i < len(request.Messages)-1; i += 2 {
                if i+1 < len(request.Messages) {
                        userMsg := request.Messages[i]
                        assistantMsg := request.Messages[i+1]
                        
                        if userMsg.Role == "user" && assistantMsg.Role == "assistant" {
                                ragService.Memory.AddDialogTurn(userMsg.Content, assistantMsg.Content)
                        }
                }
        }
        
        // 获取查询
        query := lastMessage.Content
        
        // 初始化响应通道
        clientGone := c.Request.Context().Done()
        
        // 启动 goroutine 来处理生成和流式传输
        go func() {
                // 处理文件内容（如果提供）
                fileContent := ""
                if request.FilePath != "" {
                        content, err := data.GetFileContent(request.RepoURL, request.FilePath, accessToken)
                        if err == nil {
                                fileContent = content
                        }
                }
                
                // 获取相关文档
                contextDocs, err := ragService.RetrieveDocuments(query)
                if err != nil {
                        // 继续处理，但没有上下文
                }
                
                // 获取仓库类型和名称
                repoType := "GitHub"
                if strings.Contains(request.RepoURL, "gitlab.com") {
                        repoType = "GitLab"
                }
                
                parts := strings.Split(request.RepoURL, "/")
                repoName := parts[len(parts)-1]
                if strings.HasSuffix(repoName, ".git") {
                        repoName = repoName[:len(repoName)-4]
                }
                
                // 构建系统提示
                systemPrompt := fmt.Sprintf(`<role>
你是一位专家代码分析师，正在检查 %s 仓库: %s (%s)。
你提供有关代码仓库的直接、简洁和准确的信息。
你绝不以 markdown 标题或代码围栏开始响应。
</role>

</example_of_what_not_to_do>

- 在回答中使用适当的 markdown 格式，包括标题、列表和代码块
- 对于代码分析，使用清晰的章节组织你的回答
- 逐步思考并逻辑地构建你的回答
- 从最相关的信息开始，直接解答用户的查询
- 在讨论代码时要精确和技术性
</guidelines>
- 直接回答用户的问题，不要使用任何前言或填充短语
- 不要以"好的，这是一个分析"或"以下是解释"等前言开始
- 不要以 markdown 标题（如"## 分析..."）或任何文件路径引用开始
- 不要以 ```markdown 代码围栏开始
- 不要以 ``` 结束响应
不要重复或确认问题
直接开始回答问题
```markdown ## 分析 `adalflow/adalflow/datasets/gsm8k.py`
此文件包含...
<style>
- 使用简洁、直接的语言
- 优先考虑准确性而非冗长
- 显示代码时，在相关时包括行号和文件路径
- 使用 markdown 格式提高可读性
</style>`, repoType, request.RepoURL, repoName)
                
                // 构建提示
                prompt := systemPrompt + "\n\n"
                
                // 添加对话历史
                conversationHistory := ragService.Memory.GetFormattedHistory()
                if conversationHistory != "" {
                        prompt += "<conversation_history>\n" + conversationHistory + "</conversation_history>\n\n"
                }
                
                // 添加文件内容（如果有）
                if fileContent != "" {
                        prompt += fmt.Sprintf("<currentFileContent path=\"%s\">\n%s\n</currentFileContent>\n\n", 
                                request.FilePath, fileContent)
                }
                
                // 添加上下文（如果有）
                contextText := ""
                if len(contextDocs) > 0 {
                        for _, doc := range contextDocs {
                                contextText += doc.Text + "\n\n"
                        }
                        prompt += "<context>\n" + contextText + "</context>\n\n"
                } else {
                        prompt += "<note>没有检索增强回答。</note>\n\n"
                }
                
                // 添加查询
                prompt += "<query>\n" + query + "</query>\n\nAssistant: "
                
                // 生成响应并流式传输
                response, err := ragService.GenerateStreamingResponse(prompt)
                if err != nil {
                        // 尝试简化的提示
                        simplifiedPrompt := systemPrompt + "\n\n"
                        if conversationHistory != "" {
                                simplifiedPrompt += "<conversation_history>\n" + conversationHistory + "</conversation_history>\n\n"
                        }
                        if fileContent != "" {
                                simplifiedPrompt += fmt.Sprintf("<currentFileContent path=\"%s\">\n%s\n</currentFileContent>\n\n", 
                                        request.FilePath, fileContent)
                        }
                        simplifiedPrompt += "<note>由于输入大小限制，没有检索增强回答。</note>\n\n"
                        simplifiedPrompt += "<query>\n" + query + "</query>\n\nAssistant: "
                        
                        response, err = ragService.GenerateStreamingResponse(simplifiedPrompt)
                        if err != nil {
                                c.SSEvent("", "抱歉，我无法处理您的请求。请尝试较短的查询或将其分成较小的部分。")
                                return
                        }
                }
                
                // 将响应流式传输到客户端
                for chunk := range response {
                        select {
                        case <-clientGone:
                                return
                        default:
                                c.SSEvent("", chunk)
                                c.Writer.Flush()
                        }
                }
        }()
        
        // 等待客户端断开连接
        <-clientGone
}

// 处理 wiki 导出
func (s *Server) handleExportWiki(c *gin.Context) {
        var request models.WikiExportRequest
        
        if err := c.ShouldBindJSON(&request); err != nil {
                c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("无效请求: %v", err)})
                return
        }
        
        // 提取仓库名称和时间戳用于文件名
        repoParts := strings.Split(strings.TrimRight(request.RepoURL, "/"), "/")
        repoName := repoParts[len(repoParts)-1]
        timestamp := time.Now().Format("20060102_150405")
        
        var content string
        var filename string
        var contentType string
        
        if request.Format == "markdown" {
                // 生成 Markdown 内容
                content = generateMarkdownExport(request.RepoURL, request.Pages)
                filename = fmt.Sprintf("%s_wiki_%s.md", repoName, timestamp)
                contentType = "text/markdown"
        } else {
                // 生成 JSON 内容
                content = generateJSONExport(request.RepoURL, request.Pages)
                filename = fmt.Sprintf("%s_wiki_%s.json", repoName, timestamp)
                contentType = "application/json"
        }
        
        // 设置文件下载的头部
        c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
        c.Data(http.StatusOK, contentType, []byte(content))
}

// 生成 Markdown 导出
func generateMarkdownExport(repoURL string, pages []models.WikiPage) string {
        // 开始添加元数据
        markdown := fmt.Sprintf("# Wiki Documentation for %s\n\n", repoURL)
        markdown += fmt.Sprintf("Generated on: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))
        
        // 添加目录
        markdown += "## Table of Contents\n\n"
        for _, page := range pages {
                markdown += fmt.Sprintf("- [%s](#%s)\n", page.Title, page.ID)
        }
        markdown += "\n"
        
        // 添加每个页面
        for _, page := range pages {
                markdown += fmt.Sprintf("<a id='%s'></a>\n\n", page.ID)
                markdown += fmt.Sprintf("## %s\n\n", page.Title)
                
                // 添加相关文件
                if len(page.FilePaths) > 0 {
                        markdown += "### Related Files\n\n"
                        for _, filePath := range page.FilePaths {
                                markdown += fmt.Sprintf("- `%s`\n", filePath)
                        }
                        markdown += "\n"
                }
                
                // 添加相关页面
                if len(page.RelatedPages) > 0 {
                        markdown += "### Related Pages\n\n"
                        relatedTitles := []string{}
                        for _, relatedID := range page.RelatedPages {
                                // 查找相关页面的标题
                                for _, p := range pages {
                                        if p.ID == relatedID {
                                                relatedTitles = append(relatedTitles, fmt.Sprintf("[%s](#%s)", p.Title, p.ID))
                                                break
                                        }
                                }
                        }
                        
                        if len(relatedTitles) > 0 {
                                markdown += "Related topics: " + strings.Join(relatedTitles, ", ") + "\n\n"
                        }
                }
                
                // 添加页面内容
                markdown += fmt.Sprintf("%s\n\n", page.Content)
                markdown += "---\n\n"
        }
        
        return markdown
}

// 生成 JSON 导出
func generateJSONExport(repoURL string, pages []models.WikiPage) string {
        // 使用标准库的 JSON 编码
        type ExportData struct {
                Metadata struct {
                        Repository  string    `json:"repository"`
                        GeneratedAt time.Time `json:"generated_at"`
                        PageCount   int       `json:"page_count"`
                } `json:"metadata"`
                Pages []models.WikiPage `json:"pages"`
        }
        
        data := ExportData{
                Pages: pages,
        }
        data.Metadata.Repository = repoURL
        data.Metadata.GeneratedAt = time.Now()
        data.Metadata.PageCount = len(pages)
        
        jsonData, err := json.MarshalIndent(data, "", "  ")
        if err != nil {
                return fmt.Sprintf("Error generating JSON: %v", err)
        }
        
        return string(jsonData)
}