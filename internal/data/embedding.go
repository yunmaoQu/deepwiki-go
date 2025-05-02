// internal/data/embedding.go
package data

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/deepwiki-go/internal/config"
	"github.com/deepwiki-go/internal/models"
)

// EmbeddingService 提供文本嵌入功能
type EmbeddingService struct {
	config *config.Config
	client *http.Client
}

// NewEmbeddingService 创建新的嵌入服务
func NewEmbeddingService(cfg *config.Config) *EmbeddingService {
	return &EmbeddingService{
		config: cfg,
		client: &http.Client{},
	}
}

// openAIEmbeddingRequest OpenAI API 嵌入请求结构
type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

// openAIEmbeddingResponse OpenAI API 嵌入响应结构
type openAIEmbeddingResponse struct {
	Data  []embeddingData `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type embeddingData struct {
	Embedding []float32 `json:"embedding"`
}

// GetEmbeddings 使用OpenAI获取文本的嵌入向量
func (e *EmbeddingService) GetEmbeddings(texts []string) ([][]float32, error) {
	if e.config.OpenAIAPIKey == "" {
		return nil, errors.New("未设置OpenAI API密钥")
	}

	// 准备请求数据
	reqData := openAIEmbeddingRequest{
		Model: "text-embedding-ada-002", // 使用OpenAI嵌入模型
		Input: texts,
	}

	jsonData, err := json.Marshal(reqData)
	if err != nil {
		return nil, fmt.Errorf("序列化请求失败: %v", err)
	}

	// 创建HTTP请求
	req, err := http.NewRequest("POST", "https://api.openai.com/v1/embeddings", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.config.OpenAIAPIKey))

	// 发送请求
	resp, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API请求失败: %v", err)
	}
	defer resp.Body.Close()

	// 读取响应
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 解析响应
	var embeddingResp openAIEmbeddingResponse
	if err := json.Unmarshal(body, &embeddingResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}

	// 检查错误
	if embeddingResp.Error != nil {
		return nil, fmt.Errorf("API错误: %s", embeddingResp.Error.Message)
	}

	// 提取嵌入
	embeddings := make([][]float32, len(embeddingResp.Data))
	for i, data := range embeddingResp.Data {
		embeddings[i] = data.Embedding
	}

	return embeddings, nil
}

// CreateDocumentEmbeddings 为文档创建嵌入
func (e *EmbeddingService) CreateDocumentEmbeddings(docs []models.Document) ([]models.Document, error) {
	if len(docs) == 0 {
		return docs, nil
	}

	// 准备文本切片
	var texts []string
	for _, doc := range docs {
		texts = append(texts, doc.Text)
	}

	// 获取嵌入
	embeddings, err := e.GetEmbeddings(texts)
	if err != nil {
		return nil, err
	}

	// 更新文档
	for i, embedding := range embeddings {
		if i < len(docs) {
			docs[i].Vector = embedding
		}
	}

	return docs, nil
}

// SplitText 将文本分割为块
func (e *EmbeddingService) SplitText(text string) []string {
	// 根据配置的分割方式和块大小进行分割
	splitBy := e.config.TextSplitter.SplitBy
	chunkSize := e.config.TextSplitter.ChunkSize
	overlap := e.config.TextSplitter.ChunkOverlap

	var chunks []string

	if splitBy == "word" {
		// 按单词分割
		words := strings.Fields(text)
		for i := 0; i < len(words); i += chunkSize - overlap {
			end := i + chunkSize
			if end > len(words) {
				end = len(words)
			}
			chunks = append(chunks, strings.Join(words[i:end], " "))
			if end == len(words) {
				break
			}
		}
	} else {
		// 按行分割 (默认)
		lines := strings.Split(text, "\n")
		var currentChunk []string
		currentLength := 0

		for _, line := range lines {
			lineWords := len(strings.Fields(line))
			if currentLength+lineWords > chunkSize && len(currentChunk) > 0 {
				// 当前块已满，添加到chunks并开始新块
				chunks = append(chunks, strings.Join(currentChunk, "\n"))

				// 处理重叠
				if overlap > 0 && len(currentChunk) > overlap {
					currentChunk = currentChunk[len(currentChunk)-overlap:]
					currentLength = 0
					for _, l := range currentChunk {
						currentLength += len(strings.Fields(l))
					}
				} else {
					currentChunk = []string{}
					currentLength = 0
				}
			}

			currentChunk = append(currentChunk, line)
			currentLength += lineWords
		}

		// 添加最后一个块
		if len(currentChunk) > 0 {
			chunks = append(chunks, strings.Join(currentChunk, "\n"))
		}
	}

	return chunks
}
