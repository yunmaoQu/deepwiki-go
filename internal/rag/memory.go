// internal/rag/memory.go
package rag

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/deepwiki-go/internal/models"
	"github.com/google/uuid"
)

// Memory 管理对话历史
type Memory struct {
	dialogTurns []models.DialogTurn
	mutex       sync.RWMutex
}

// NewMemory 创建一个新的内存实例
func NewMemory() *Memory {
	return &Memory{
		dialogTurns: make([]models.DialogTurn, 0),
	}
}

// AddDialogTurn 向对话历史添加一个对话轮次
func (m *Memory) AddDialogTurn(userQuery string, assistantResponse string) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	log.Printf("AddDialogTurn userQuery: %s, assistantResponse: %s", userQuery, assistantResponse)

	turn := models.DialogTurn{
		ID:                uuid.New().String(),
		UserQuery:         userQuery,
		AssistantResponse: assistantResponse,
	}

	m.dialogTurns = append(m.dialogTurns, turn)
}

// GetDialogTurns 返回所有对话轮次
func (m *Memory) GetDialogTurns() []models.DialogTurn {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// 返回副本以避免并发修改
	turns := make([]models.DialogTurn, len(m.dialogTurns))
	copy(turns, m.dialogTurns)

	return turns
}

// GetFormattedHistory 返回格式化的对话历史
func (m *Memory) GetFormattedHistory() string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.dialogTurns) == 0 {
		return ""
	}

	var history string
	for _, turn := range m.dialogTurns {
		history += fmt.Sprintf("<turn>\n<user>%s</user>\n<assistant>%s</assistant>\n</turn>\n",
			turn.UserQuery, turn.AssistantResponse)
	}

	return history
}

// GetRelevantContext 获取与当前查询相关的上下文信息
func (m *Memory) GetRelevantContext(query string) string {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	if len(m.dialogTurns) == 0 {
		return ""
	}

	// 获取最近的几轮对话作为上下文
	maxTurns := 3
	startIdx := 0
	if len(m.dialogTurns) > maxTurns {
		startIdx = len(m.dialogTurns) - maxTurns
	}

	// 检查是否有与当前查询相关的历史对话
	relevantTurns := make([]models.DialogTurn, 0)
	queryLower := strings.ToLower(query)

	// 首先尝试查找精确相关的对话轮次
	for i := startIdx; i < len(m.dialogTurns); i++ {
		turn := m.dialogTurns[i]
		if similarityScore(queryLower, strings.ToLower(turn.UserQuery)) > 0.3 {
			relevantTurns = append(relevantTurns, turn)
		}
	}

	// 如果没有找到相关轮次，返回最近的对话
	if len(relevantTurns) == 0 {
		relevantTurns = m.dialogTurns[startIdx:]
	}

	// 构建上下文字符串
	var context strings.Builder
	for _, turn := range relevantTurns {
		context.WriteString(fmt.Sprintf("问题: %s\n回答: %s\n\n", turn.UserQuery, turn.AssistantResponse))
	}

	return context.String()
}

// similarityScore 计算两个字符串的相似度分数
func similarityScore(a, b string) float64 {
	// 简单的关键词匹配算法
	aWords := strings.Fields(a)
	bWords := strings.Fields(b)

	matches := 0
	for _, aWord := range aWords {
		for _, bWord := range bWords {
			if aWord == bWord && len(aWord) > 1 {
				matches++
				break
			}
		}
	}

	// 计算相似度分数
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}

	// 使用 Jaccard 相似度
	return float64(matches) / float64(len(aWords)+len(bWords)-matches)
}

// Clear 清除内存中的所有对话轮次
func (m *Memory) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.dialogTurns = make([]models.DialogTurn, 0)
}
