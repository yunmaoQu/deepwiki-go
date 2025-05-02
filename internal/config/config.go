package config

import "os"

// Config 保存应用程序配置
type Config struct {
        GoogleAPIKey string
        OpenAIAPIKey string
        ProjectID    string // Google Cloud 项目 ID
        Location     string // Google Cloud 区域
        Port         string
        
        // 文本分割器配置
        TextSplitter struct {
                SplitBy      string
                ChunkSize    int
                ChunkOverlap int
        }
        
        // 检索器配置
        Retriever struct {
                TopK int
        }
        
        // 排除的目录和文件
        FileFilters struct {
                ExcludedDirs  []string
                ExcludedFiles []string
        }
}

// NewConfig 创建并返回一个新的配置实例
func NewConfig() *Config {
        cfg := &Config{
                GoogleAPIKey: os.Getenv("GOOGLE_API_KEY"),
                OpenAIAPIKey: os.Getenv("OPENAI_API_KEY"),
                ProjectID:    os.Getenv("GOOGLE_CLOUD_PROJECT"),
                Location:     os.Getenv("GOOGLE_CLOUD_LOCATION"),
                Port:         os.Getenv("PORT"),
        }
        
        // 设置默认值
        cfg.TextSplitter.SplitBy = "word"
        cfg.TextSplitter.ChunkSize = 350
        cfg.TextSplitter.ChunkOverlap = 100
        
        cfg.Retriever.TopK = 20
        
        // 设置默认排除的目录和文件
        cfg.FileFilters.ExcludedDirs = []string{
                ".venv", "venv", "node_modules", ".git", "__pycache__",
                ".pytest_cache", "dist", "build", "docs", ".idea", ".vscode",
        }
        
        cfg.FileFilters.ExcludedFiles = []string{
                "package-lock.json", "yarn.lock", "poetry.lock", "Pipfile.lock",
                ".DS_Store", "Thumbs.db", ".env", ".gitignore",
        }
        
        return cfg
}