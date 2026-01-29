package database

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"forum/models"

	_ "github.com/mattn/go-sqlite3"
)

// SessionsMu protects concurrent access to the in-memory session store.
var SessionsMu sync.RWMutex

// Sessions хранит сессии пользователей.
var Sessions = make(map[string]models.SessionData)

// InitDB открывает или создаёт базу данных и выполняет миграции схемы.
func InitDB() (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "./forum.db?_foreign_keys=on")
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		return nil, err
	}
	if err := ensureSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// ensureSchema создаёт необходимые таблицы и базовые данные, если они отсутствуют.
func ensureSchema(db *sql.DB) error {
	statements := []string{
		`PRAGMA foreign_keys = ON;`,
		`CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			role TEXT NOT NULL DEFAULT 'user',
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);`,
		`CREATE TABLE IF NOT EXISTS sessions (
			session_id TEXT PRIMARY KEY,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL,
			expiry DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS posts (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			title TEXT NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			image_url TEXT,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS categories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL UNIQUE
		);`,
		`CREATE TABLE IF NOT EXISTS post_categories (
			post_id INTEGER NOT NULL,
			category_id INTEGER NOT NULL,
			PRIMARY KEY(post_id, category_id),
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY(category_id) REFERENCES categories(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS post_votes (
			user_id INTEGER NOT NULL,
			post_id INTEGER NOT NULL,
			vote INTEGER NOT NULL CHECK(vote IN (-1, 1)),
			PRIMARY KEY(user_id, post_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			post_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			content TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY(post_id) REFERENCES posts(id) ON DELETE CASCADE,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS comment_votes (
			user_id INTEGER NOT NULL,
			comment_id INTEGER NOT NULL,
			vote INTEGER NOT NULL CHECK(vote IN (-1, 1)),
			PRIMARY KEY(user_id, comment_id),
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
			FOREIGN KEY(comment_id) REFERENCES comments(id) ON DELETE CASCADE
		);`,
	}

	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("schema migration failed: %w", err)
		}
	}

	categories := []string{"news", "life", "auto", "creative", "gadgets", "science", "games", "other"}
	for _, name := range categories {
		if _, err := db.Exec("INSERT OR IGNORE INTO categories (name) VALUES (?)", name); err != nil {
			return fmt.Errorf("seed categories failed: %w", err)
		}
	}

	// Ensure the display_name column exists in users table (for full name display).
	// Ignore error if the column already exists.
	_, _ = db.Exec("ALTER TABLE users ADD COLUMN display_name TEXT")

	return nil
}

// GetSessionData возвращает userID, роль и срок действия сессии по sessionID.
// В случае отсутствия сессии или ошибки возвращает нулевые значения и ошибку.
func GetSessionData(db *sql.DB, sessionID string) (int, string, time.Time, error) {
	var userID int
	var role string
	var expiry time.Time
	err := db.QueryRow("SELECT user_id, role, expiry FROM sessions WHERE session_id = ?", sessionID).Scan(&userID, &role, &expiry)
	if err != nil {
		return 0, "", time.Time{}, err
	}
	return userID, role, expiry, nil
}

// DeleteExpiredSession удаляет истёкшую сессию из базы данных.
// Логирует ошибку, если удаление не удалось.
func DeleteExpiredSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID)
	if err != nil {
		log.Println("Error deleting expired session:", err)
	}
	return err
}

// DeleteSession удаляет сессию из базы данных и из памяти.
// Возвращает ошибку, если удаление из базы не удалось.
func DeleteSession(db *sql.DB, sessionID string) error {
	_, err := db.Exec("DELETE FROM sessions WHERE session_id = ?", sessionID)
	if err != nil {
		log.Println("Error deleting session from database:", err)
		return err
	}
	delete(Sessions, sessionID)
	return nil
}

// GetUsernameByID возвращает имя пользователя по его ID.
// В случае отсутствия пользователя возвращает пустую строку и ошибку.
func GetUsernameByID(db *sql.DB, userID int) (string, error) {
	var username string
	err := db.QueryRow("SELECT username FROM users WHERE id = ?", userID).Scan(&username)
	if err != nil {
		return "", err
	}
	return username, nil
}

// GetDisplayName returns the display_name for a user by ID.
func GetDisplayName(db *sql.DB, userID int) (string, error) {
	var displayName sql.NullString
	err := db.QueryRow("SELECT display_name FROM users WHERE id = ?", userID).Scan(&displayName)
	if err != nil {
		return "", err
	}
	if displayName.Valid {
		return displayName.String, nil
	}
	return "", nil
}

// GetUserByEmail возвращает ID, имя, хэш пароля и роль пользователя по email.
// В случае отсутствия пользователя возвращает нулевые значения и ошибку.
func GetUserByEmail(db *sql.DB, email string) (int, string, string, string, error) {
	var userID int
	var username, hashedPassword, role string
	err := db.QueryRow("SELECT id, username, password, role FROM users WHERE email = ?", email).Scan(&userID, &username, &hashedPassword, &role)
	if err != nil {
		return 0, "", "", "", err
	}
	return userID, username, hashedPassword, role, nil
}

// GetUserProfileData возвращает имя пользователя и дату создания профиля по ID.
// В случае отсутствия пользователя возвращает пустые строки и ошибку.
func GetUserProfileData(db *sql.DB, userID int) (string, time.Time, error) {
	var username string
	var createdAt time.Time
	err := db.QueryRow("SELECT username, created_at FROM users WHERE id = ?", userID).Scan(&username, &createdAt)
	if err != nil {
		return "", time.Now(), err
	}
	return username, createdAt, nil
}

// EmailExists проверяет, существует ли email в базе пользователей.
// Возвращает true, если email существует, иначе false.
func EmailExists(db *sql.DB, email string) (bool, error) {
	var exists string
	err := db.QueryRow("SELECT email FROM users WHERE email = ?", email).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// UsernameExists проверяет, существует ли имя пользователя в базе.
// Возвращает true, если имя существует, иначе false.
func UsernameExists(db *sql.DB, username string) (bool, error) {
	username = strings.ToLower(username)

	var exists bool
	err := db.QueryRow(
		"SELECT EXISTS(SELECT 1 FROM users WHERE LOWER(username) = ?)",
		username,
	).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// RegisterUser создаёт нового пользователя с указанным email, именем и хэшем пароля.
// Присваивает роль "user". Возвращает ошибку, если регистрация не удалась.
func RegisterUser(db *sql.DB, email, username, hashedPassword string) error {
	_, err := db.Exec("INSERT INTO users (email, username, password, role) VALUES (?, ?, ?, 'user')", email, username, hashedPassword)
	return err
}

// DeleteUserSessions удаляет все сессии пользователя из базы данных.
// Возвращает ошибку, если удаление не удалось.
func DeleteUserSessions(db *sql.DB, userID int) error {
	_, err := db.Exec("DELETE FROM sessions WHERE user_id = ?", userID)
	return err
}

// UpdateUserProfile updates username and display_name for a user.
func UpdateUserProfile(db *sql.DB, userID int, username string, displayName string) error {
	_, err := db.Exec("UPDATE users SET username = ?, display_name = ? WHERE id = ?", username, displayName, userID)
	return err
}

// CreateSession создаёт новую сессию с указанным ID, userID, ролью и сроком действия.
// Возвращает ошибку, если создание не удалось.
func CreateSession(db *sql.DB, sessionID string, userID int, role string, expiry time.Time) error {
	_, err := db.Exec("INSERT INTO sessions (session_id, user_id, role, expiry) VALUES (?, ?, ?, ?)", sessionID, userID, role, expiry)
	return err
}

// GetPostByIDAndUserID возвращает данные поста по его ID и ID пользователя.
// В случае отсутствия поста возвращает пустую структуру и ошибку.
func GetPostByIDAndUserID(db *sql.DB, postID int, userID int) (models.PostData, error) {
	var post models.PostData
	err := db.QueryRow(`
        SELECT id, title, content, user_id, image_url
        FROM posts WHERE id = ? AND user_id = ?
    `, postID, userID).Scan(&post.ID, &post.Title, &post.Content, &post.UserID, &post.ImageURL)
	if err != nil {
		return models.PostData{}, err
	}
	return post, nil
}

// GetPostOwnerID возвращает ID владельца поста по ID поста.
// В случае отсутствия поста возвращает 0 и ошибку.
func GetPostOwnerID(db *sql.DB, postID int) (int, error) {
	var ownerID int
	err := db.QueryRow("SELECT user_id FROM posts WHERE id = ?", postID).Scan(&ownerID)
	if err != nil {
		return 0, err
	}
	return ownerID, nil
}

// GetUserPosts возвращает список постов пользователя с количеством лайков и дизлайков.
// Сортирует посты по дате создания (от новых к старым).
func GetUserPosts(db *sql.DB, userID int) ([]models.PostData, error) {
	query := `
        SELECT p.id, p.title, p.content, p.created_at, p.image_url,
               COALESCE(SUM(CASE WHEN pv.vote = 1 THEN 1 ELSE 0 END), 0) as likes,
               COALESCE(SUM(CASE WHEN pv.vote = -1 THEN 1 ELSE 0 END), 0) as dislikes
        FROM posts p
        LEFT JOIN post_votes pv ON p.id = pv.post_id
        WHERE p.user_id = ?
        GROUP BY p.id, p.title, p.content, p.created_at, p.image_url
        ORDER BY p.created_at DESC
    `
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []models.PostData
	for rows.Next() {
		var p models.PostData
		var imageURL sql.NullString
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &imageURL, &p.Likes, &p.Dislikes); err != nil {
			return nil, err
		}
		p.ImageURL = imageURL.String
		p.UserID = userID
		posts = append(posts, p)
	}
	return posts, nil
}

// CreatePost создаёт новый пост и возвращает его ID.
// В случае ошибки возвращает 0 и ошибку.
func CreatePost(db *sql.DB, userID int, title, content, imageURL string, createdAt time.Time) (int64, error) {
	result, err := db.Exec(
		"INSERT INTO posts (user_id, title, content, image_url, created_at) VALUES (?, ?, ?, ?, ?)",
		userID, title, content, imageURL, createdAt,
	)
	if err != nil {
		return 0, err
	}
	postID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return postID, nil
}

// UpdatePost обновляет заголовок, содержимое и URL изображения поста.
// Возвращает ошибку, если обновление не удалось.
func UpdatePost(db *sql.DB, postID int, title, content, imageURL string) error {
	_, err := db.Exec("UPDATE posts SET title = ?, content = ?, image_url = ? WHERE id = ?", title, content, imageURL, postID)
	return err
}

// DeletePost удаляет пост по его ID.
// Возвращает ошибку, если удаление не удалось.
func DeletePost(db *sql.DB, postID int) error {
	_, err := db.Exec("DELETE FROM posts WHERE id = ?", postID)
	return err
}

// GetPostCategories возвращает список категорий, связанных с постом.
// В случае ошибки возвращает nil и ошибку.
func GetPostCategories(db *sql.DB, postID int) ([]string, error) {
	rows, err := db.Query(`
        SELECT c.name FROM categories c
        JOIN post_categories pc ON c.id = pc.category_id
        WHERE pc.post_id = ?
    `, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var catName string
		if err := rows.Scan(&catName); err != nil {
			return nil, err
		}
		categories = append(categories, catName)
	}
	return categories, nil
}

// GetCategoryIDByName возвращает ID категории по её имени.
// В случае отсутствия категории возвращает 0 и ошибку.
func GetCategoryIDByName(db *sql.DB, catName string) (int, error) {
	var catID int
	err := db.QueryRow("SELECT id FROM categories WHERE name = ?", catName).Scan(&catID)
	if err != nil {
		return 0, err
	}
	return catID, nil
}

// AddPostCategory связывает пост с категорией по их ID.
// Возвращает ошибку, если операция не удалась.
func AddPostCategory(db *sql.DB, postID int64, catID int) error {
	_, err := db.Exec("INSERT INTO post_categories (post_id, category_id) VALUES (?, ?)", postID, catID)
	return err
}

// DeletePostCategories удаляет все категории, связанные с постом.
// Возвращает ошибку, если удаление не удалось.
func DeletePostCategories(db *sql.DB, postID int) error {
	_, err := db.Exec("DELETE FROM post_categories WHERE post_id = ?", postID)
	return err
}

// DeletePostComments удаляет все комментарии к посту.
// Возвращает ошибку, если удаление не удалось.
func DeletePostComments(db *sql.DB, postID int) error {
	_, err := db.Exec("DELETE FROM comments WHERE post_id = ?", postID)
	return err
}

// DeletePostVotes удаляет все лайки и дизлайки поста.
// Возвращает ошибку, если удаление не удалось.
func DeletePostVotes(db *sql.DB, postID int) error {
	_, err := db.Exec("DELETE FROM post_votes WHERE post_id = ?", postID)
	return err
}

// GetUserPostVote возвращает голос пользователя за пост (1, -1 или 0).
// Если голоса нет, возвращает 0 и false. При ошибке возвращает 0, false и ошибку.
func GetUserPostVote(db *sql.DB, userID, postID int) (int64, bool, error) {
	var currentVote sql.NullInt64
	err := db.QueryRow("SELECT vote FROM post_votes WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&currentVote)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return currentVote.Int64, true, nil
}

// RemovePostVote удаляет голос пользователя за пост.
// Возвращает ошибку, если удаление не удалось.
func RemovePostVote(db *sql.DB, userID, postID int) error {
	_, err := db.Exec("DELETE FROM post_votes WHERE user_id = ? AND post_id = ?", userID, postID)
	return err
}

// SetPostLike устанавливает или обновляет лайк пользователя для поста.
// Возвращает ошибку, если операция не удалась.
func SetPostLike(db *sql.DB, userID, postID int) error {
	_, err := db.Exec(`
        INSERT INTO post_votes (user_id, post_id, vote) VALUES (?, ?, 1)
        ON CONFLICT(user_id, post_id) DO UPDATE SET vote = 1
    `, userID, postID)
	return err
}

// SetPostDislike устанавливает или обновляет дизлайк пользователя для поста.
// Возвращает ошибку, если операция не удалась.
func SetPostDislike(db *sql.DB, userID, postID int) error {
	_, err := db.Exec(`
        INSERT INTO post_votes (user_id, post_id, vote) VALUES (?, ?, -1)
        ON CONFLICT(user_id, post_id) DO UPDATE SET vote = -1
    `, userID, postID)
	return err
}

// GetPostVoteStats возвращает количество лайков, дизлайков и голос пользователя для поста.
// Если голоса пользователя нет, возвращает 0 и false для userVote.
func GetPostVoteStats(db *sql.DB, userID, postID int) (int, int, int64, bool, error) {
	var likes, dislikes int
	var userVote sql.NullInt64
	err := db.QueryRow(`
        SELECT COALESCE(SUM(CASE WHEN vote = 1 THEN 1 ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN vote = -1 THEN 1 ELSE 0 END), 0),
               (SELECT vote FROM post_votes WHERE user_id = ? AND post_id = ?)
        FROM post_votes WHERE post_id = ?
    `, userID, postID, postID).Scan(&likes, &dislikes, &userVote)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if userVote.Valid {
		return likes, dislikes, userVote.Int64, true, nil
	}
	return likes, dislikes, 0, false, nil
}

// CreateComment создаёт новый комментарий к посту и возвращает его ID.
// В случае ошибки возвращает 0 и ошибку.
func CreateComment(db *sql.DB, postID int, userID int, content, createdAt string) (int64, error) {
	result, err := db.Exec(`
		INSERT INTO comments (post_id, user_id, content, created_at)
		VALUES (?, ?, ?, ?)`,
		postID, userID, content, createdAt,
	)
	if err != nil {
		return 0, err
	}
	commentID, err := result.LastInsertId()
	if err != nil {
		return 0, err
	}
	return commentID, nil
}

// GetCommentsByPostIDWithUserVote возвращает комментарии к посту с лайками, дизлайками и голосом текущего пользователя.
// Сортирует комментарии по дате создания (от новых к старым).
func GetCommentsByPostIDWithUserVote(db *sql.DB, currentUserID, postID int) ([]models.CommentData, error) {
	query := `
        SELECT c.id, c.content, c.created_at, u.id, u.username,
               COALESCE(SUM(CASE WHEN cv.vote = 1 THEN 1 ELSE 0 END), 0) as likes,
               COALESCE(SUM(CASE WHEN cv.vote = -1 THEN 1 ELSE 0 END), 0) as dislikes,
               (SELECT cv2.vote FROM comment_votes cv2 WHERE cv2.comment_id = c.id AND cv2.user_id = ?) as user_vote
        FROM comments c
        JOIN users u ON c.user_id = u.id
        LEFT JOIN comment_votes cv ON c.id = cv.comment_id
        WHERE c.post_id = ?
        GROUP BY c.id, c.content, c.created_at, u.id, u.username
        ORDER BY c.created_at DESC
    `
	rows, err := db.Query(query, currentUserID, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []models.CommentData
	for rows.Next() {
		var c models.CommentData
		var userVote sql.NullInt64
		if err := rows.Scan(&c.ID, &c.Content, &c.CreatedAt, &c.UserID, &c.Username, &c.Likes, &c.Dislikes, &userVote); err != nil {
			return nil, err
		}
		if userVote.Valid {
			c.UserVote = int(userVote.Int64)
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// DeleteComment удаляет комментарий по его ID.
// Возвращает ошибку, если удаление не удалось.
func DeleteComment(db *sql.DB, commentID int) error {
	_, err := db.Exec("DELETE FROM comments WHERE id = ?", commentID)
	return err
}

// DeleteCommentVotes удаляет все лайки и дизлайки комментария.
// Возвращает ошибку, если удаление не удалось.
func DeleteCommentVotes(db *sql.DB, commentID int) error {
	_, err := db.Exec("DELETE FROM comment_votes WHERE comment_id = ?", commentID)
	return err
}

// GetUserCommentVote возвращает голос пользователя за комментарий (1, -1 или 0).
// Если голоса нет, возвращает 0 и false. При ошибке возвращает 0, false и ошибку.
func GetUserCommentVote(db *sql.DB, userID, commentID int) (int64, bool, error) {
	var currentVote sql.NullInt64
	err := db.QueryRow("SELECT vote FROM comment_votes WHERE user_id = ? AND comment_id = ?", userID, commentID).Scan(&currentVote)
	if err == sql.ErrNoRows {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return currentVote.Int64, true, nil
}

// RemoveCommentVote удаляет голос пользователя за комментарий.
// Возвращает ошибку, если удаление не удалось.
func RemoveCommentVote(db *sql.DB, userID, commentID int) error {
	_, err := db.Exec("DELETE FROM comment_votes WHERE user_id = ? AND comment_id = ?", userID, commentID)
	return err
}

// SetCommentLike устанавливает или обновляет лайк пользователя для комментария.
// Возвращает ошибку, если операция не удалась.
func SetCommentLike(db *sql.DB, userID, commentID int) error {
	_, err := db.Exec(`
        INSERT INTO comment_votes (user_id, comment_id, vote) VALUES (?, ?, 1)
        ON CONFLICT(user_id, comment_id) DO UPDATE SET vote = 1
    `, userID, commentID)
	return err
}

// SetCommentDislike устанавливает или обновляет дизлайк пользователя для комментария.
// Возвращает ошибку, если операция не удалась.
func SetCommentDislike(db *sql.DB, userID, commentID int) error {
	_, err := db.Exec(`
        INSERT INTO comment_votes (user_id, comment_id, vote) VALUES (?, ?, -1)
        ON CONFLICT(user_id, comment_id) DO UPDATE SET vote = -1
    `, userID, commentID)
	return err
}

// GetCommentVoteStats возвращает количество лайков, дизлайков и голос пользователя для комментария.
// Если голоса пользователя нет, возвращает 0 и false для userVote.
func GetCommentVoteStats(db *sql.DB, userID, commentID int) (int, int, int64, bool, error) {
	var likes, dislikes int
	var userVote sql.NullInt64
	err := db.QueryRow(`
        SELECT COALESCE(SUM(CASE WHEN vote = 1 THEN 1 ELSE 0 END), 0),
               COALESCE(SUM(CASE WHEN vote = -1 THEN 1 ELSE 0 END), 0),
               (SELECT vote FROM comment_votes WHERE user_id = ? AND comment_id = ?)
        FROM comment_votes WHERE comment_id = ?
    `, userID, commentID, commentID).Scan(&likes, &dislikes, &userVote)
	if err != nil {
		return 0, 0, 0, false, err
	}
	if userVote.Valid {
		return likes, dislikes, userVote.Int64, true, nil
	}
	return likes, dislikes, 0, false, nil
}

// GetPosts возвращает список постов с учётом фильтра (my, liked, commented, best, new) и категории.
// Включает лайки, дизлайки, голос пользователя и категории поста.
func GetPosts(db *sql.DB, userID int, filter, category string) ([]models.PostData, error) {
	query := `
        SELECT p.id, p.title, p.content, p.created_at, p.image_url, p.user_id, u.username,
               COALESCE(SUM(CASE WHEN pv.vote = 1 THEN 1 ELSE 0 END), 0) AS likes,
               COALESCE(SUM(CASE WHEN pv.vote = -1 THEN 1 ELSE 0 END), 0) AS dislikes,
               COALESCE(pv_user.vote, 0) AS user_vote,
               GROUP_CONCAT(c.name) AS categories
        FROM posts p
        JOIN users u ON p.user_id = u.id
        LEFT JOIN post_votes pv ON p.id = pv.post_id
        LEFT JOIN post_votes pv_user ON p.id = pv_user.post_id AND pv_user.user_id = ?
        LEFT JOIN post_categories pc ON p.id = pc.post_id
        LEFT JOIN categories c ON pc.category_id = c.id
    `
	args := []interface{}{userID}

	var orderBy string
	switch filter {
	case "my":
		query += " WHERE p.user_id = ?"
		args = append(args, userID)
		orderBy = " ORDER BY p.created_at DESC"
	case "liked":
		query += " WHERE EXISTS (SELECT 1 FROM post_votes pv2 WHERE pv2.post_id = p.id AND pv2.user_id = ? AND pv2.vote = 1)"
		args = append(args, userID)
		orderBy = " ORDER BY p.created_at DESC"
	case "commented":
		query += " WHERE EXISTS (SELECT 1 FROM comments c WHERE c.post_id = p.id AND c.user_id = ?)"
		args = append(args, userID)
		orderBy = " ORDER BY p.created_at DESC"
	case "best":
		orderBy = " ORDER BY (COALESCE(SUM(CASE WHEN pv.vote = 1 THEN 1 ELSE 0 END), 0) - COALESCE(SUM(CASE WHEN pv.vote = -1 THEN 1 ELSE 0 END), 0)) DESC"
	case "new":
		orderBy = " ORDER BY p.created_at DESC"
	default:
		filter = "new"
		orderBy = " ORDER BY p.created_at DESC"
	}

	if category != "" {
		if filter == "new" || filter == "best" {
			query += " WHERE c.name = ?"
		} else {
			query += " AND c.name = ?"
		}
		args = append(args, category)
	}

	query += " GROUP BY p.id, p.title, p.content, p.created_at, p.image_url, p.user_id, pv_user.vote" + orderBy

	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var posts []models.PostData
	for rows.Next() {
		var p models.PostData
		var imageURL sql.NullString
		var categories sql.NullString
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &imageURL, &p.UserID, &p.Username, &p.Likes, &p.Dislikes, &p.UserVote, &categories); err != nil {
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		p.ImageURL = imageURL.String
		if categories.Valid {
			p.Categories = strings.Split(categories.String, ",")
		}
		if len(p.Categories) > 0 {
			p.Category = p.Categories[0]
		}
		posts = append(posts, p)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %v", err)
	}

	return posts, nil
}

// GetCommentsByPostID возвращает список комментариев к посту с лайками и дизлайками.
// Сортирует комментарии по дате создания (от старых к новым).
func GetCommentsByPostID(db *sql.DB, userID, postID int) ([]models.CommentData, error) {
	query := `
        SELECT c.id, c.post_id, c.user_id, u.username, c.content, c.created_at,
               COALESCE(SUM(CASE WHEN cv.vote = 1 THEN 1 ELSE 0 END), 0) AS likes,
               COALESCE(SUM(CASE WHEN cv.vote = -1 THEN 1 ELSE 0 END), 0) AS dislikes
        FROM comments c
        JOIN users u ON c.user_id = u.id
        LEFT JOIN comment_votes cv ON c.id = cv.comment_id
        WHERE c.post_id = ?
        GROUP BY c.id, c.post_id, c.user_id, u.username, c.content, c.created_at
        ORDER BY c.created_at ASC
    `
	rows, err := db.Query(query, postID)
	if err != nil {
		return nil, fmt.Errorf("query failed: %v", err)
	}
	defer rows.Close()

	var comments []models.CommentData
	for rows.Next() {
		var c models.CommentData
		if err := rows.Scan(&c.ID, &c.PostID, &c.UserID, &c.Username, &c.Content, &c.CreatedAt, &c.Likes, &c.Dislikes); err != nil {
			return nil, fmt.Errorf("scan failed: %v", err)
		}
		comments = append(comments, c)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %v", err)
	}

	return comments, nil
}

// GetPostByID возвращает данные поста по его ID, включая лайки, дизлайки, голос пользователя и категории.
// В случае отсутствия поста возвращает пустую структуру и ошибку.
func GetPostByID(db *sql.DB, postID, currentUserID int) (models.PostData, error) {
	var post models.PostData
	var imageURL sql.NullString
	var categories sql.NullString

	query := `
        SELECT p.id, p.title, p.content, p.created_at, p.image_url, p.user_id, u.username,
               COALESCE(SUM(CASE WHEN pv.vote = 1 THEN 1 ELSE 0 END), 0) AS likes,
               COALESCE(SUM(CASE WHEN pv.vote = -1 THEN 1 ELSE 0 END), 0) AS dislikes,
               COALESCE(pv_user.vote, 0) AS user_vote,
               GROUP_CONCAT(c.name) AS categories
        FROM posts p
        JOIN users u ON p.user_id = u.id
        LEFT JOIN post_votes pv ON p.id = pv.post_id
        LEFT JOIN post_votes pv_user ON p.id = pv_user.post_id AND pv_user.user_id = ?
        LEFT JOIN post_categories pc ON p.id = pc.post_id
        LEFT JOIN categories c ON pc.category_id = c.id
        WHERE p.id = ?
        GROUP BY p.id, p.title, p.content, p.created_at, p.image_url, p.user_id, u.username, pv_user.vote
    `

	err := db.QueryRow(query, currentUserID, postID).Scan(
		&post.ID, &post.Title, &post.Content, &post.CreatedAt, &imageURL,
		&post.UserID, &post.Username, &post.Likes, &post.Dislikes, &post.UserVote, &categories,
	)
	if err != nil {
		return models.PostData{}, err
	}

	post.ImageURL = imageURL.String
	if categories.Valid {
		post.Categories = strings.Split(categories.String, ",")
	}
	if len(post.Categories) > 0 {
		post.Category = post.Categories[0]
	}

	return post, nil
}

// GetCommentOwnerID возвращает ID владельца комментария по его ID.
// В случае отсутствия комментария возвращает 0 и ошибку.
func GetCommentOwnerID(db *sql.DB, commentID int) (int, error) {
	var ownerID int
	err := db.QueryRow("SELECT user_id FROM comments WHERE id = ?", commentID).Scan(&ownerID)
	if err != nil {
		return 0, err
	}
	return ownerID, nil
}
