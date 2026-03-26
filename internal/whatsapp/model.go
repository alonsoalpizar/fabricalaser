package whatsapp

import "time"

// WhatsappConversation es el modelo GORM para la tabla whatsapp_conversations.
// Usado únicamente para referencia — las escrituras se hacen via pgAdapter con SQL directo.
type WhatsappConversation struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Phone     string    `gorm:"not null"   json:"phone"`
	Role      string    `gorm:"not null"   json:"role"`
	Content   string    `gorm:"not null"   json:"content"`
	CreatedAt time.Time `gorm:"not null"   json:"created_at"`
}

func (WhatsappConversation) TableName() string {
	return "whatsapp_conversations"
}
