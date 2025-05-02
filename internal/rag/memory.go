// internal/rag/memory.go
package rag

import (
        "fmt"
        "sync"
	"log"
        "github.com/google/uuid"
        "github.com/deepwiki-go/internal/models"
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

// Clear 清除内存中的所有对话轮次
func (m *Memory) Clear() {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	m.dialogTurns = make([]models.DialogTurn, 0)
}