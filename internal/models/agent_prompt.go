package models

import "time"

// AgentPrompt es la fila activa del prompt de un agente.
// Cada agent_key aparece una sola vez en esta tabla (UNIQUE index).
type AgentPrompt struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AgentKey    string    `gorm:"column:agent_key;type:varchar(50);uniqueIndex;not null" json:"agent_key"`
	Title       string    `gorm:"column:title;type:varchar(100);not null" json:"title"`
	Description *string   `gorm:"column:description;type:text" json:"description,omitempty"`
	Body        string    `gorm:"column:body;type:text;not null;default:''" json:"body"`
	Version     int       `gorm:"column:version;not null;default:1" json:"version"`
	UpdatedBy   *uint     `gorm:"column:updated_by" json:"updated_by,omitempty"`
	IsActive    bool      `gorm:"column:is_active;not null;default:true" json:"is_active"`
	CreatedAt   time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (AgentPrompt) TableName() string { return "agent_prompts" }

// AgentPromptVersion es una versión histórica. Se crea una fila cada vez que se guarda
// el body del prompt. RestoredFromVersion != nil indica que fue resultado de un rollback.
type AgentPromptVersion struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	AgentPromptID       uint      `gorm:"column:agent_prompt_id;not null" json:"agent_prompt_id"`
	Version             int       `gorm:"column:version;not null" json:"version"`
	Body                string    `gorm:"column:body;type:text;not null" json:"body"`
	UpdatedBy           *uint     `gorm:"column:updated_by" json:"updated_by,omitempty"`
	Note                *string   `gorm:"column:note;type:text" json:"note,omitempty"`
	RestoredFromVersion *int      `gorm:"column:restored_from_version" json:"restored_from_version,omitempty"`
	CreatedAt           time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AgentPromptVersion) TableName() string { return "agent_prompt_versions" }
