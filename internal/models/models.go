// internal/models/models.go
package models

// ChatMessage 表示聊天消息
type ChatMessage struct {
	Role    string `json:"role"`    // 'user' 或 'assistant'
	Content string `json:"content"` // 消息内容
}

// ChatCompletionRequest 表示聊天完成请求
type ChatCompletionRequest struct {
	RepoURL     string        `json:"repo_url"`               // 仓库 URL
	Messages    []ChatMessage `json:"messages"`               // 聊天消息列表
	FilePath    string        `json:"filePath,omitempty"`     // 可选的文件路径
	GitHubToken string        `json:"github_token,omitempty"` // GitHub 访问令牌
	GitLabToken string        `json:"gitlab_token,omitempty"` // GitLab 访问令牌
}

// Document 表示一个文档
type Document struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Text     string    `json:"text"`
	MetaData map[string]interface{} `json:"meta_data"`
	Vector   []float32 `json:"vector,omitempty"`
	Importance   string             `json:"importance"`
	RelatedPages []string           `json:"related_pages,omitempty"`
}

// WikiPage 表示一个Wiki页面
type WikiPage struct {
	ID           string   `json:"id"`
	Title        string   `json:"title"`
	Content      string   `json:"content"`
	FilePaths    []string `json:"file_paths,omitempty"`
	Importance   string   `json:"importance"`
	RelatedPages []string `json:"related_pages,omitempty"`
}

// WikiExportRequest 表示 wiki 导出请求
type WikiExportRequest struct {
	RepoURL string     `json:"repo_url"`
	Pages   []WikiPage `json:"pages"`
	Format  string     `json:"format"` // "markdown" 或 "json"
}

// DialogTurn 表示对话轮次
type DialogTurn struct {
	ID                string `json:"id"`
	UserQuery         string `json:"user_query"`
	AssistantResponse string `json:"assistant_response"`
}

// RAGResult 表示 RAG 结果
type RAGResult struct {
	Rationale string `json:"rationale"`
	Answer    string `json:"answer"`
}
