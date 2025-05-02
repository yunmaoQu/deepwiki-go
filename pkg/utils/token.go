// pkg/utils/tokens.go
package utils

import (
	"log"

	"github.com/pkoukk/tiktoken-go"
)

// 最大嵌入 token 限制
const MaxEmbeddingTokens = 8192

// CountTokens 使用OpenAI tiktoken分词器精确计算token数
func CountTokens(text string, model string) int {
	enc, err := tiktoken.EncodingForModel(model)
	if err != nil {
		log.Printf("tiktoken: 模型不支持，使用cl100k_base: %v", err)
		enc, _ = tiktoken.GetEncoding("cl100k_base")
	}
	tokens := enc.Encode(text, nil, nil)
	count := len(tokens)
	log.Printf("Token count for text (model: %s): %d", model, count)
	return count
}
