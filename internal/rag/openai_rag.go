package rag

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

	"bufio"
	"strings"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/internal/data"
	"github.com/deepwiki-go/internal/models"
)

// OpenAIRAG 实现基于 OpenAI 的检索增强生成
type OpenAIRAG struct {
	Memory       *Memory
	Config       *config.Config
	DbManager    *data.DatabaseManager
	RepoURL      string
	OpenAIClient *http.Client 
}

// NewOpenAIRAG 创建一个新的 OpenAI RAG 实例
func NewOpenAIRAG(cfg *config.Config) (*OpenAIRAG, error) {
	dbManager, err := data.NewDatabaseManager(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create DatabaseManager: %w", err)
	}
	return &OpenAIRAG{
		Memory:    NewMemory(),
		Config:    cfg,
		DbManager: dbManager,
	}, nil
}

// Name 返回提供者的唯一名称
func (r *OpenAIRAG) Name() string {
	return "openai"
}

// Initialize 初始化提供者
func (r *OpenAIRAG) Initialize() error {
	if r.Config.OpenAIAPIKey == "" {
		return fmt.Errorf("缺少必要的 OpenAI API Key")
	}
	// 初始化OpenAI客户端
	r.OpenAIClient = &http.Client{}
	return nil
}

// PrepareRetriever 为仓库准备检索器
func (r *OpenAIRAG) PrepareRetriever(repoURLOrPath string, accessToken string) error {
	r.RepoURL = repoURLOrPath
	err := r.DbManager.PrepareDatabase(repoURLOrPath, accessToken)
	if err != nil {
		return fmt.Errorf("failed to prepare database: %w", err)
	}

	log.Println("Database prepared for retrieval")
	return nil
}

// RetrieveDocuments 检索与查询相关的文档
func (r *OpenAIRAG) RetrieveDocuments(query string) ([]models.Document, error) {
	// 使用向量搜索检索相关文档
	relevantDocs, err := r.DbManager.SearchDocuments(query, r.Config.Retriever.TopK)
	if err != nil {
		return nil, err
	}

	return relevantDocs, nil
}

// GenerateStreamingResponse 生成流式响应
func (r *OpenAIRAG) GenerateStreamingResponse(prompt string) (chan string, error) {
	if r.OpenAIClient == nil {
		return nil, errors.New("OpenAI 客户端未初始化")
	}

	responseCh := make(chan string)
	go func() {
		defer close(responseCh)

		// 构造OpenAI Chat Completion流式请求
		url := "https://api.openai.com/v1/chat/completions"
		requestBody := map[string]interface{}{
			"model": "gpt-o3", 
			"messages": []map[string]string{{
				"role":    "user",
				"content": prompt,
			}},
			"stream": true,
		}
		jsonData, _ := json.Marshal(requestBody)

		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
		if err != nil {
			responseCh <- "请求构建失败"
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+r.Config.OpenAIAPIKey)

		resp, err := r.OpenAIClient.Do(req)
		if err != nil {
			responseCh <- "请求发送失败: " + err.Error()
			return
		}
		defer resp.Body.Close()

		reader := bufio.NewReader(resp.Body)
		for {
			line, err := reader.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					break
				}
				responseCh <- "读取响应失败: " + err.Error()
				break
			}
			line = strings.TrimSpace(line)
			if line == "" || !strings.HasPrefix(line, "data:") {
				continue
			}
			if line == "data: [DONE]" {
				break
			}
			// 解析JSON块
			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &chunk); err == nil {
				for _, choice := range chunk.Choices {
					if choice.Delta.Content != "" {
						responseCh <- choice.Delta.Content
					}
				}
			}
		}
	}()
	return responseCh, nil
}

// Close 清理资源
func (r *OpenAIRAG) Close() error {
	r.OpenAIClient = nil
	return nil
}

// IndexDocument 索引文档
func (r *OpenAIRAG) IndexDocument(doc *models.Document) error {
	if r.DbManager == nil {
		return errors.New("数据库管理器未初始化")
	}
	return r.DbManager.AddDocument(doc)
}

// GetDocument 获取文档
func (r *OpenAIRAG) GetDocument(id string) (*models.Document, error) {
	if r.DbManager == nil {
		return nil, errors.New("数据库管理器未初始化")
	}
	return r.DbManager.GetDocument(id)
}

// DeleteDocument 删除文档
func (r *OpenAIRAG) DeleteDocument(id string) error {
	if r.DbManager == nil {
		return errors.New("数据库管理器未初始化")
	}
	return r.DbManager.DeleteDocument(id)
}
