// Package main содержит настройку маршрутов для приложения форума.
// Регистрирует обработчики HTTP-запросов и возвращает кастомный обработчик.

package main

import (
	"database/sql"
	"net/http"

	"forum/handlers"
)

// setupRoutes настраивает маршруты приложения и возвращает HTTP-обработчик.
// Регистрирует обработчики для статических файлов и основных маршрутов, оборачивает их в CustomHandler.
func setupRoutes(db *sql.DB) http.Handler {
	mux := http.NewServeMux()

	// Обслуживает статические файлы из директорий static и images.
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	// Исправлено: изображения теперь обслуживаются из static/images
	mux.Handle("/images/", http.StripPrefix("/images/", http.FileServer(http.Dir("static/images"))))

	// Регистрирует обработчики для основных маршрутов
	mux.HandleFunc("/", handlers.IndexHandler(db))
	mux.HandleFunc("/register", handlers.RegisterHandler(db))
	mux.HandleFunc("/login", handlers.LoginHandler(db))
	mux.HandleFunc("/logout", handlers.LogoutHandler(db))
	mux.HandleFunc("/profile", handlers.ProfileHandler(db))
	mux.HandleFunc("/post", handlers.PostHandler(db))
	mux.HandleFunc("/create-post", handlers.CreatePostHandler(db))
	mux.HandleFunc("/edit-post", handlers.EditPostHandler(db))
	mux.HandleFunc("/delete-post", handlers.DeletePostHandler(db))
	mux.HandleFunc("/delete-comment", handlers.DeleteCommentHandler(db))
	mux.HandleFunc("/like", handlers.LikeHandler(db))
	mux.HandleFunc("/dislike", handlers.DislikeHandler(db))
	mux.HandleFunc("/comment", handlers.CommentHandler(db))
	mux.HandleFunc("/comment-like", handlers.CommentLikeHandler(db))
	mux.HandleFunc("/comment-dislike", handlers.CommentDislikeHandler(db))
	mux.HandleFunc("/update-profile", handlers.UpdateProfileHandler(db))

	// Оборачивает маршрутизатор в CustomHandler для обработки паник и ошибок 404.
	return &CustomHandler{mux: mux}
}
