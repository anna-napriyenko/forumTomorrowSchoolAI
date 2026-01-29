package handlers

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/database"
)

// CommentHandler создаёт новый комментарий к посту.
// Принимает POST-запрос с post_id и content, возвращает JSON с данными комментария или ошибкой.
// Требует аутентификации пользователя.
func CommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			log.Printf("Unauthenticated user attempted to create a comment.")
			http.Redirect(w, r, "/?message=Login+please", http.StatusSeeOther)
			return
		}

		if r.Method != "POST" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Method not allowed.",
			})
			return
		}

		if err := r.ParseForm(); err != nil {
			log.Printf("Error parsing form: %v.", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Bad request.",
			})
			return
		}

		log.Printf("Received form data: %v.", r.Form)

		postIDStr := r.FormValue("post_id")
		content := r.FormValue("content")
		log.Printf("Comment attempt: post_id=%s, content=%q.", postIDStr, content)

		if postIDStr == "" || content == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Post ID and content are required.",
			})
			return
		}

		postID, err := strconv.Atoi(postIDStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Invalid Post ID.",
			})
			return
		}

		trimmedContent := strings.TrimSpace(content)
		if trimmedContent == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment content cannot be empty or contain only whitespace.",
			})
			return
		}

		if len(trimmedContent) < 3 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment must be at least 3 characters long.",
			})
			return
		}
		if len(trimmedContent) > 500 {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment cannot be longer than 500 characters.",
			})
			return
		}

		createdAt := time.Now().Format("2006-01-02 15:04:05")
		commentID, err := database.CreateComment(db, postID, userID, content, createdAt)
		if err != nil {
			log.Println("Error inserting comment:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		username, err := database.GetUsernameByID(db, userID)
		if err != nil {
			log.Println("Error fetching username:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success":    true,
			"comment_id": commentID,
			"content":    content,
			"user_id":    userID,
			"username":   username,
			"created_at": createdAt,
		})
	}
}

// DeleteCommentHandler удаляет комментарий по его ID.
// Принимает DELETE-запрос, требует аутентификации и прав администратора или владельца комментария.
// Возвращает JSON с результатом операции.
func DeleteCommentHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			log.Println("Method not allowed:", r.Method)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Allow", "DELETE")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Method not allowed.",
			})
			return
		}

		isAuth, userID, role := IsAuthenticated(db, r)
		if !isAuth {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Not authenticated.",
			})
			return
		}

		commentIDStr := r.URL.Query().Get("comment_id")
		if commentIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment ID is required.",
			})
			return
		}
		commentID, err := strconv.Atoi(commentIDStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Invalid Comment ID.",
			})
			return
		}

		commentOwnerID, err := database.GetCommentOwnerID(db, commentID)
		if err == sql.ErrNoRows {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment not found.",
			})
			return
		}

		if role != "admin" && userID != commentOwnerID {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Unauthorized.",
			})
			return
		}

		if err != nil {
			log.Println("Error fetching comment owner:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		err = database.DeleteCommentVotes(db, commentID)
		if err != nil {
			log.Println("Error deleting comment votes:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		err = database.DeleteComment(db, commentID)
		if err != nil {
			log.Println("Error deleting comment:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		log.Printf("User %d deleted comment %d successfully.", userID, commentID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Comment deleted.",
		})
	}
}

// CommentLikeHandler устанавливает или снимает лайк для комментария.
// Принимает POST-запрос с comment_id, требует аутентификации.
// Возвращает JSON с количеством лайков, дизлайков и текущим голосом пользователя.
func CommentLikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			log.Printf("Unauthenticated user attempted to like a comment.")
			http.Redirect(w, r, "/?message=Login+please", http.StatusSeeOther)
			return
		}

		if r.Method != "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Method not allowed.",
			})
			return
		}

		commentIDStr := r.URL.Query().Get("comment_id")
		if commentIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment ID is required.",
			})
			return
		}
		commentID, err := strconv.Atoi(commentIDStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Invalid Comment ID.",
			})
			return
		}

		currentVote, voteExists, err := database.GetUserCommentVote(db, userID, commentID)
		if err != nil {
			log.Println("Error checking vote:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if voteExists && currentVote == 1 {
			err = database.RemoveCommentVote(db, userID, commentID)
		} else {
			err = database.SetCommentLike(db, userID, commentID)
		}
		if err != nil {
			log.Println("Error updating vote:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		likes, dislikes, userVote, userVoteExists, err := database.GetCommentVoteStats(db, userID, commentID)
		if err != nil {
			log.Println("Error fetching comment votes:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"success":   true,
			"likes":     likes,
			"dislikes":  dislikes,
			"user_vote": int64(0),
		}
		if userVoteExists {
			response["user_vote"] = userVote
		}
		json.NewEncoder(w).Encode(response)
	}
}

// CommentDislikeHandler устанавливает или снимает дизлайк для комментария.
// Принимает POST-запрос с comment_id, требует аутентификации.
// Возвращает JSON с количеством лайков, дизлайков и текущим голосом пользователя.
func CommentDislikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			log.Printf("Unauthenticated user attempted to dislike a comment.")
			http.Redirect(w, r, "/?message=Login+please", http.StatusSeeOther)
			return
		}

		if r.Method != "POST" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Method not allowed.",
			})
			return
		}

		commentIDStr := r.URL.Query().Get("comment_id")
		if commentIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Comment ID is required.",
			})
			return
		}
		commentID, err := strconv.Atoi(commentIDStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Invalid Comment ID.",
			})
			return
		}

		currentVote, voteExists, err := database.GetUserCommentVote(db, userID, commentID)
		if err != nil {
			log.Println("Error checking vote:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if voteExists && currentVote == -1 {
			err = database.RemoveCommentVote(db, userID, commentID)
		} else {
			err = database.SetCommentDislike(db, userID, commentID)
		}
		if err != nil {
			log.Println("Error updating vote:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		likes, dislikes, userVote, userVoteExists, err := database.GetCommentVoteStats(db, userID, commentID)
		if err != nil {
			log.Println("Error fetching comment votes:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		response := map[string]interface{}{
			"success":   true,
			"likes":     likes,
			"dislikes":  dislikes,
			"user_vote": int64(0),
		}
		if userVoteExists {
			response["user_vote"] = userVote
		}
		json.NewEncoder(w).Encode(response)
	}
}
