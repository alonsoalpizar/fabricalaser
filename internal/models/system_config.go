package models

import (
	"time"
)

type SystemConfig struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ConfigKey   string    `gorm:"column:config_key;type:varchar(100);uniqueIndex;not null" json:"config_key"`
	ConfigValue string    `gorm:"column:config_value;type:text;not null" json:"config_value"`
	ValueType   string    `gorm:"column:value_type;type:varchar(20);not null;default:'string'" json:"value_type"`
	Category    string    `gorm:"column:category;type:varchar(50);not null" json:"category"`
	Description *string   `gorm:"type:text" json:"description,omitempty"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (SystemConfig) TableName() string {
	return "system_config"
}
