package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"forum/database"
	"forum/models"
)

// IndexHandler отображает главную страницу с постами.
// Принимает GET-запрос с параметрами filter и category, возвращает HTML-страницу.
// Перенаправляет неаутентифицированных пользователей на логин для фильтров my, liked, commented.
func IndexHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			return
		}

		if r.Method != "GET" {
			log.Println("Method not allowed:", r.Method)
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Method not allowed.")
			return
		}

		isAuth, userID, role := IsAuthenticated(db, r)
		var username string
		if isAuth {
			var err error
			username, err = database.GetUsernameByID(db, userID)
			if err != nil {
				log.Println("Error fetching username:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
		}

		message := r.URL.Query().Get("message")
		filter := r.URL.Query().Get("filter")
		if filter == "" {
			filter = "new"
		}
		category := r.URL.Query().Get("category")
		log.Printf("Filter applied: %s, Category: %s.", filter, category)

		validFilters := map[string]bool{
			"new": true, "best": true, "my": true, "liked": true, "commented": true,
		}
		if !validFilters[filter] {
			log.Printf("Invalid filter value: %s.", filter)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "Invalid filter value.")
			return
		}

		validCategories := map[string]bool{
			"news": true, "life": true, "auto": true, "creative": true,
			"gadgets": true, "science": true, "games": true, "other": true,
		}
		if category != "" && !validCategories[category] {
			log.Printf("Invalid category value: %s.", category)
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintln(w, "Invalid category value.")
			return
		}

		posts, err := database.GetPosts(db, userID, filter, category)
		if err != nil {
			log.Println("Error querying posts:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}
		log.Printf("Posts retrieved: %d.", len(posts))
		for i, p := range posts {
			likes, dislikes, userVote, _, _ := database.GetPostVoteStats(db, userID, p.ID)
			posts[i].Likes = likes
			posts[i].Dislikes = dislikes
			posts[i].UserVote = int(userVote)
			posts[i].CreatedAtStr = posts[i].CreatedAt.Format(time.DateOnly)
			log.Printf("Post %d: ID=%d, Likes=%d, Dislikes=%d.", i, p.ID, p.Likes, p.Dislikes)
		}

		if (filter == "my" || filter == "liked" || filter == "commented") && !isAuth {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		for i := range posts {
			comments, err := database.GetCommentsByPostIDWithUserVote(db, userID, posts[i].ID)
			if err != nil {
				log.Println("Error querying comments:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			posts[i].Comments = comments
		}

		tmpl, err := template.ParseFiles("templates/index.html")
		if err != nil {
			log.Println("Error parsing template:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		data := models.PageData{
			IsAuthenticated: isAuth,
			UserID:          userID,
			Username:        username,
			Role:            role,
			Posts:           posts,
			ErrorMessage:    r.URL.Query().Get("login_error"),
			Filter:          filter,
			Message:         message,
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("Error executing template:", err)
		}
	}
}

// CreatePostHandler создаёт новый пост.
// При GET отображает форму создания, при POST сохраняет пост с категориями.
// Требует аутентификации, перенаправляет на логин при её отсутствии.
func CreatePostHandler(db *sql.DB) http.HandlerFunc {
	allowedCategories := map[string]bool{
		"news": true, "gadgets": true, "life": true, "auto": true,
		"creative": true, "science": true, "games": true, "other": true,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, role := IsAuthenticated(db, r)
		if !isAuth {
			http.Redirect(w, r, "/login?redirect=/create-post", http.StatusSeeOther)
			return
		}

		username, err := database.GetUsernameByID(db, userID)
		if err != nil {
			log.Println("Error fetching username:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		fmt.Println(r.Method)
		if r.Method == "GET" {
			tmpl, err := template.ParseFiles("templates/create_post.html")
			if err != nil {
				log.Println("Error parsing create post template:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			pageData := models.PageData{
				IsAuthenticated: isAuth,
				UserID:          userID,
				Username:        username,
				Role:            role,
				ErrorMessage:    r.URL.Query().Get("error"),
			}
			if err := tmpl.Execute(w, pageData); err != nil {
				log.Println("Error executing create post template:", err)
				writeError(w, http.StatusInternalServerError)
			}
			return
		}

		if r.Method != "POST" {
			err := ErrorTpl.Execute(
				w,
				struct {
					Code    int
					Message string
				}{
					405, "Method not allowed",
				},
			)
			if err != nil {
				http.Error(w, "Method not allowd", 405)
			}
			return
		}

		if err := r.ParseForm(); err != nil {
			log.Println("Error parsing form:", err)
			http.Redirect(w, r, "/create-post?error=Bad+request", http.StatusSeeOther)
			return
		}

		title := strings.TrimSpace(r.FormValue("title"))
		content := strings.TrimSpace(r.FormValue("content"))
		imageURL := r.FormValue("image_url")
		categories := r.Form["categories"]

		if title == "" || content == "" {
			http.Redirect(w, r, "/create-post?error=Title+and+content+cannot+be+empty", http.StatusSeeOther)
			return
		}

		validCategories := make([]string, 0, len(categories))
		for _, catName := range categories {
			catNameLower := strings.ToLower(catName)
			if allowedCategories[catNameLower] {
				validCategories = append(validCategories, catNameLower)
			}
		}
		if len(validCategories) == 0 {
			http.Redirect(w, r, "/create-post?error=Please+choose+valid+category", http.StatusSeeOther)
			return
		}
		if len(validCategories) > 3 {
			http.Redirect(w, r, "/create-post?error=You+can+select+up+to+3+categories", http.StatusSeeOther)
			return
		}

		createdAt := time.Now()
		postID, err := database.CreatePost(db, userID, title, content, imageURL, createdAt)
		if err != nil {
			log.Println("Error inserting post:", err)
			http.Redirect(w, r, "/create-post?error=Server+error", http.StatusSeeOther)
			return
		}

		for _, catName := range validCategories {
			catID, err := database.GetCategoryIDByName(db, catName)
			if err != nil {
				log.Println("Error fetching category:", err)
				http.Redirect(w, r, "/create-post?error=Server+error", http.StatusSeeOther)
				return
			}
			err = database.AddPostCategory(db, postID, catID)
			if err != nil {
				log.Println("Error inserting post_category:", err)
				http.Redirect(w, r, "/create-post?error=Server+error", http.StatusSeeOther)
				return
			}
		}
		http.Redirect(w, r, "/post?post_id="+strconv.FormatInt(postID, 10), http.StatusSeeOther)
		return

	}
}

// EditPostHandler редактирует существующий пост.
// При GET отображает форму редактирования, при POST обновляет пост и категории.
// Требует аутентификации и прав владельца поста.
func EditPostHandler(db *sql.DB) http.HandlerFunc {
	allowedCategories := map[string]bool{
		"news": true, "gadgets": true, "life": true, "auto": true,
		"creative": true, "science": true, "games": true, "other": true,
	}
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, role := IsAuthenticated(db, r)
		if !isAuth {
			http.Redirect(w, r, "/login?redirect=/edit-post?post_id="+r.URL.Query().Get("post_id"), http.StatusSeeOther)
			return
		}

		username, err := database.GetUsernameByID(db, userID)
		if err != nil {
			log.Println("Error fetching username:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		if r.Method == "GET" {
			postIDStr := r.URL.Query().Get("post_id")
			if postIDStr == "" {
				writeError(w, http.StatusBadRequest)
				return
			}
			postID, err := strconv.Atoi(postIDStr)
			if err != nil {
				writeError(w, http.StatusBadRequest)
				return
			}

			post, err := database.GetPostByIDAndUserID(db, postID, userID)
			if err == sql.ErrNoRows {
				writeError(w, http.StatusForbidden)
				return
			}
			if err != nil {
				log.Println("Error fetching post:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			post.Categories, err = database.GetPostCategories(db, postID)
			if err != nil {
				log.Println("Error fetching categories:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			tmpl, err := template.ParseFiles("templates/edit_post.html")
			if err != nil {
				log.Println("Error parsing edit post template:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			pageData := models.PageData{
				IsAuthenticated: isAuth,
				UserID:          userID,
				Username:        username,
				Role:            role,
				Post:            post,
				ErrorMessage:    r.URL.Query().Get("error"),
			}
			if err := tmpl.Execute(w, pageData); err != nil {
				log.Println("Error executing edit post template:", err)
				writeError(w, http.StatusInternalServerError)
			}
			return
		}

		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				log.Println("Error parsing form:", err)
				writeError(w, http.StatusBadRequest)
				return
			}

			postIDStr := r.FormValue("post_id")
			if postIDStr == "" {
				writeError(w, http.StatusBadRequest)
				return
			}
			postID, err := strconv.Atoi(postIDStr)
			if err != nil {
				writeError(w, http.StatusBadRequest)
				return
			}

			ownerID, err := database.GetPostOwnerID(db, postID)
			if err == sql.ErrNoRows {
				writeError(w, http.StatusNotFound)
				return
			}
			if err != nil {
				log.Println("Error fetching post owner:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			if ownerID != userID {
				writeError(w, http.StatusForbidden)
				return
			}

			title := strings.TrimSpace(r.FormValue("title"))
			content := strings.TrimSpace(r.FormValue("content"))
			imageURL := r.FormValue("image_url")
			categories := r.Form["categories"]

			if title == "" || content == "" {
				writeError(w, http.StatusBadRequest)
				return
			}

			validCategories := make([]string, 0, len(categories))
			for _, catName := range categories {
				catNameLower := strings.ToLower(catName)
				if allowedCategories[catNameLower] {
					validCategories = append(validCategories, catNameLower)
				}
			}
			if len(validCategories) == 0 {
				writeError(w, http.StatusBadRequest)
				return
			}
			if len(validCategories) > 2 {
				writeError(w, http.StatusBadRequest)
				return
			}

			err = database.UpdatePost(db, postID, title, content, imageURL)
			if err != nil {
				log.Println("Error updating post:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			err = database.DeletePostCategories(db, postID)
			if err != nil {
				log.Println("Error deleting categories:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			for _, catName := range validCategories {
				catID, err := database.GetCategoryIDByName(db, catName)
				if err == sql.ErrNoRows {
					log.Printf("Category %s not found in allowed list.", catName)
					writeError(w, http.StatusBadRequest)
					return
				}
				if err != nil {
					log.Println("Error fetching category:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
				err = database.AddPostCategory(db, int64(postID), catID)
				if err != nil {
					log.Println("Error inserting post_category:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
			}

			http.Redirect(w, r, "/?filter=my", http.StatusSeeOther)
			return
		}

		writeError(w, http.StatusMethodNotAllowed)
	}
}

// DeletePostHandler удаляет пост по его ID.
// Принимает DELETE-запрос, требует аутентификации и прав администратора или владельца.
// Возвращает JSON с результатом операции.
func DeletePostHandler(db *sql.DB) http.HandlerFunc {
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

		postIDStr := r.URL.Query().Get("post_id")
		if postIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Post ID is required.",
			})
			return
		}
		postID, err := strconv.Atoi(postIDStr)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Invalid Post ID.",
			})
			return
		}

		postUserID, err := database.GetPostOwnerID(db, postID)
		if err != nil {
			log.Println("Error fetching post:", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Post not found.",
			})
			return
		}

		if userID != postUserID && role != "admin" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Unauthorized.",
			})
			return
		}

		if err := database.DeletePostCategories(db, postID); err != nil {
			log.Println("Error deleting post categories:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if err := database.DeletePostComments(db, postID); err != nil {
			log.Println("Error deleting comments:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if err := database.DeletePostVotes(db, postID); err != nil {
			log.Println("Error deleting post votes:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if err := database.DeletePost(db, postID); err != nil {
			log.Println("Error deleting post:", err)
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		log.Printf("User %d deleted post %d.", userID, postID)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"message": "Post deleted successfully.",
		})
	}
}

// LikeHandler устанавливает или снимает лайк для поста.
// Принимает POST-запрос с post_id, требует аутентификации.
// Возвращает JSON с количеством лайков, дизлайков и текущим голосом пользователя.
func LikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Not authenticated.",
			})
			return
		}

		postIDStr := r.URL.Query().Get("post_id")
		if postIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Post ID is required.",
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

		currentVote, voteExists, err := database.GetUserPostVote(db, userID, postID)
		if err != nil {
			log.Println("Error checking vote:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if voteExists && currentVote == 1 {
			err = database.RemovePostVote(db, userID, postID)
		} else {
			err = database.SetPostLike(db, userID, postID)
		}
		if err != nil {
			log.Println("Error updating vote:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		likes, dislikes, userVote, userVoteExists, err := database.GetPostVoteStats(db, userID, postID)
		if err != nil {
			log.Println("Error fetching votes:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
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

// DislikeHandler устанавливает или снимает дизлайк для поста.
// Принимает POST-запрос с post_id, требует аутентификации.
// Возвращает JSON с количеством лайков, дизлайков и текущим голосом пользователя.
func DislikeHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Not authenticated.",
			})
			return
		}

		postIDStr := r.URL.Query().Get("post_id")
		if postIDStr == "" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Post ID is required.",
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

		currentVote, voteExists, err := database.GetUserPostVote(db, userID, postID)
		if err != nil {
			log.Println("Error checking vote:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		if voteExists && currentVote == -1 {
			err = database.RemovePostVote(db, userID, postID)
		} else {
			err = database.SetPostDislike(db, userID, postID)
		}
		if err != nil {
			log.Println("Error updating vote:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		likes, dislikes, userVote, userVoteExists, err := database.GetPostVoteStats(db, userID, postID)
		if err != nil {
			log.Println("Error fetching votes:", err)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "Server error.",
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
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

// PostHandler отображает страницу отдельного поста с комментариями.
// Принимает GET-запрос с post_id, возвращает HTML-страницу.
// Возвращает ошибку, если пост не найден.
func PostHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			log.Println("Method not allowed:", r.Method)
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Allow", "GET")
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintln(w, "Method not allowed.")
			return
		}

		postIDStr := r.URL.Query().Get("post_id")
		if postIDStr == "" {
			writeError(w, http.StatusBadRequest)
			return
		}
		postID, err := strconv.Atoi(postIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest)
			return
		}

		isAuth, userID, role := IsAuthenticated(db, r)
		var username string
		if isAuth {
			username, err = database.GetUsernameByID(db, userID)
			if err != nil {
				log.Println("Error fetching username:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
		}

		post, err := database.GetPostByID(db, postID, userID)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusBadRequest)
			return
		}
		post.CreatedAtStr = post.CreatedAt.Format(time.DateOnly)
		likes, dislikes, userVote, _, _ := database.GetPostVoteStats(db, userID, postID)
		post.Likes = likes
		post.Dislikes = dislikes
		post.UserVote = int(userVote)

		if err != nil {
			log.Println("Error fetching post:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		comments, err := database.GetCommentsByPostIDWithUserVote(db, userID, postID)
		if err != nil {
			log.Println("Error querying comments:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}
		post.Comments = comments
		
		for i := range post.Comments {
			c := &comments[i]
			c.CreatedAtStr = c.CreatedAt.Format(time.DateOnly)
		}

		tmpl, err := template.ParseFiles("templates/post.html")
		if err != nil {
			log.Println("Error parsing post template:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		data := models.PageData{
			IsAuthenticated: isAuth,
			UserID:          userID,
			Username:        username,
			Role:            role,
			Post:            post,
			ErrorMessage:    r.URL.Query().Get("error"),
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, data); err != nil {
			log.Println("Error executing post template:", err)
			writeError(w, http.StatusInternalServerError)
		}
	}
}
