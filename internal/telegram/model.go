package telegram

// Update es la estructura raíz que Telegram envía en cada webhook notification.
type Update struct {
	UpdateID int64      `json:"update_id"`
	Message  *TGMessage `json:"message,omitempty"`
}

// TGMessage representa un mensaje entrante de Telegram.
type TGMessage struct {
	MessageID int64         `json:"message_id"`
	From      *TGUser       `json:"from,omitempty"`
	Chat      TGChat        `json:"chat"`
	Date      int64         `json:"date"`
	Text      string        `json:"text,omitempty"`
	Photo     []TGPhotoSize `json:"photo,omitempty"`
	Caption   string        `json:"caption,omitempty"`
}

// TGUser contiene datos del remitente.
type TGUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// TGChat identifica el chat donde se recibió el mensaje.
type TGChat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// TGPhotoSize representa una resolución de una foto enviada por el usuario.
type TGPhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int    `json:"file_size,omitempty"`
}
