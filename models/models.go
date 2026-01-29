package models

import "time"

// User представляет данные пользователя.
// Содержит идентификатор, email, имя, хешированный пароль и роль.
type User struct {
	ID       int
	Email    string
	Username string
	Password string
	Role     string
}

// SessionData хранит информацию о сессии пользователя.
// Содержит идентификатор пользователя, роль и время истечения.
type SessionData struct {
	UserID int
	Role   string
	Expiry time.Time
	
}

// Post представляет данные поста.
// Содержит идентификатор, автора, заголовок, содержимое, дату создания, URL изображения и категории.
type Post struct {
	ID         int
	UserID     int
	Title      string
	Content    string
	CreatedAt  time.Time
	ImageURL   string
	Category   string
	Categories []string
}

// Comment представляет данные комментария.
// Содержит идентификатор, автора, пост, содержимое и дату создания.
type Comment struct {
	ID        int
	UserID    int
	PostID    int
	Content   string
	CreatedAt string
}

// PostData используется для отображения поста с дополнительной информацией.
// Содержит данные поста, автора, лайки, дизлайки, комментарии и голос пользователя.
type PostData struct {
	ID           int
	Title        string
	Content      string
	CreatedAt    time.Time
	CreatedAtStr string
	UserID       int
	Username     string
	Likes        int
	Dislikes     int
	Comments     []CommentData
	ImageURL     string
	Category     string
	Categories   []string
	UserVote     int
}

// CommentData используется для отображения комментария с дополнительной информацией.
// Содержит данные комментария, автора, лайки, дизлайки и голос пользователя.
type CommentData struct {
	ID           int
	PostID       int
	UserID       int
	Username     string
	Content      string
	CreatedAt    time.Time
	CreatedAtStr string
	Likes        int
	Dislikes     int
	UserVote     int
}

// PageData используется для передачи данных в HTML-шаблоны.
// Содержит информацию об аутентификации, постах, пользователе, фильтрах и сообщениях.
type PageData struct {
	IsAuthenticated  bool
	Posts            []PostData
	UserID           int
	Username         string
	ErrorMessage     string
	Filter           string
	Role             string
	ProfileUsername  string
	ProfileCreatedAt string
	Post             PostData
	Message          string
}
