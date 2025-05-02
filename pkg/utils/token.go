// pkg/utils/tokens.go
package utils

import (
        "log"
        "strings"
)

// 最大嵌入 token 限制
const MaxEmbeddingTokens = 8192

// CountTokens 计算文本字符串中的 token 数量
func CountTokens(text string) int {
        // 简单的近似值: 平均每个单词 1.3 个 token
        // 在实际实现中，你应该使用专门的分词器
        words := strings.Fields(text)
        count := int(float64(len(words)) * 1.3)
        log.Printf("Token count for text (length: %d words): %d\n", len(words), count)
        return count
}