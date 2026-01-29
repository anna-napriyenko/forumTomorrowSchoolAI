package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"forum/database"
	"forum/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

var ErrorTpl = template.Must(template.ParseFiles("templates/error.html"))

func writeError(wr http.ResponseWriter, code int) {
	ErrorTpl.Execute(wr, struct {
		Code    int
		Message string
	}{
		Code:    code,
		Message: http.StatusText(code),
	})
}

// UpdateProfileHandler updates username and display_name for the authenticated user.
func UpdateProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, _ := IsAuthenticated(db, r)
		if !isAuth {
			writeError(w, http.StatusUnauthorized)
			return
		}

		if r.Method != "POST" {
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		if err := r.ParseForm(); err != nil {
			log.Println("Error parsing form:", err)
			writeError(w, http.StatusBadRequest)
			return
		}

		newUsername := strings.TrimSpace(r.FormValue("username"))
		newDisplayName := strings.TrimSpace(r.FormValue("display_name"))

		currentUsername, err := database.GetUsernameByID(db, userID)
		if err != nil {
			log.Println("Error fetching current username:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		// If username is empty, keep current
		if newUsername == "" {
			newUsername = currentUsername
		} else if newUsername != currentUsername {
			// Check uniqueness
			exists, err := database.UsernameExists(db, newUsername)
			if err != nil {
				log.Println("Error checking username existence:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			if exists {
				writeError(w, http.StatusBadRequest)
				return
			}
		}

		// If no display name provided, try to keep existing
		currentDisplayName, _ := database.GetDisplayName(db, userID)
		if newDisplayName == "" {
			newDisplayName = currentDisplayName
		}

		if err := database.UpdateUserProfile(db, userID, newUsername, newDisplayName); err != nil {
			log.Println("Error updating user profile:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// IsAuthenticated проверяет, аутентифицирован ли пользователь.
// Возвращает true, userID и роль, если сессия действительна, иначе false, 0 и пустую строку.
func IsAuthenticated(db *sql.DB, r *http.Request) (bool, int, string) {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return false, 0, ""
	}

	userID, role, expiry, err := database.GetSessionData(db, cookie.Value)
	if err == sql.ErrNoRows {
		return false, 0, ""
	}
	if err != nil {
		log.Println("Error querying session:", err)
		return false, 0, ""
	}

	if expiry.Before(time.Now()) {
		err := database.DeleteExpiredSession(db, cookie.Value)
		if err != nil {
			log.Println("Error deleting expired session:", err)
		}
		return false, 0, ""
	}

	database.SessionsMu.Lock()
	database.Sessions[cookie.Value] = models.SessionData{
		UserID: userID,
		Role:   role,
		Expiry: expiry,
	}
	database.SessionsMu.Unlock()

	return true, userID, role
}

// RegisterHandler регистрирует нового пользователя.
// При GET отображает форму регистрации, при POST выполняет регистрацию.
// Перенаправляет аутентифицированных пользователей на главную страницу.
func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, userID, role := IsAuthenticated(db, r)
		if isAuth {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		if r.Method == "POST" {
			if err := r.ParseForm(); err != nil {
				log.Println("Error parsing form:", err)
				writeError(w, http.StatusBadRequest)
				return
			}
			email := strings.TrimSpace(r.FormValue("email"))
			username := strings.TrimSpace(r.FormValue("username"))
			password := r.FormValue("password")

			if email == "" || username == "" || password == "" {
				tmpl, err := template.ParseFiles("templates/register.html")
				if err != nil {
					log.Println("Error parsing register template:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
				pageData := models.PageData{ErrorMessage: "All fields are required."}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := tmpl.Execute(w, pageData); err != nil {
					log.Println("Error executing register template:", err)
				}
				return
			}

			emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
			if !emailRegex.MatchString(email) {
				tmpl, err := template.ParseFiles("templates/register.html")
				if err != nil {
					log.Println("Error parsing register template:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
				pageData := models.PageData{ErrorMessage: "Invalid email format."}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := tmpl.Execute(w, pageData); err != nil {
					log.Println("Error executing register template:", err)
				}
				return
			}

			emailExists, err := database.EmailExists(db, email)
			if err != nil {
				log.Println("Error checking email:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			if emailExists {
				tmpl, err := template.ParseFiles("templates/register.html")
				if err != nil {
					log.Println("Error parsing register template:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
				pageData := models.PageData{ErrorMessage: "Email already taken."}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := tmpl.Execute(w, pageData); err != nil {
					log.Println("Error executing register template:", err)
				}
				return
			}

			usernameExists, err := database.UsernameExists(db, username)
			if err != nil {
				log.Println("Error checking username:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			if usernameExists {
				tmpl, err := template.ParseFiles("templates/register.html")
				if err != nil {
					log.Println("Error parsing register template:", err)
					writeError(w, http.StatusInternalServerError)
					return
				}
				pageData := models.PageData{ErrorMessage: "Username already taken."}
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				if err := tmpl.Execute(w, pageData); err != nil {
					log.Println("Error executing register template:", err)
				}
				return
			}

			hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
			if err != nil {
				log.Println("Error hashing password:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			err = database.RegisterUser(db, email, username, string(hashedPassword))
			if err != nil {
				log.Println("Error inserting user:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			tmpl, err := template.ParseFiles("templates/register.html")
			if err != nil {
				log.Println("Error parsing register template:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			pageData := models.PageData{Message: "Registration successful, please login."}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			if err := tmpl.Execute(w, pageData); err != nil {
				log.Println("Error executing register template:", err)
			}
			return
		}

		tmpl, err := template.ParseFiles("templates/register.html")
		if err != nil {
			log.Println("Error parsing register template:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}
		pageData := models.PageData{
			IsAuthenticated: isAuth,
			UserID:          userID,
			Username:        "",
			Role:            role,
			ErrorMessage:    r.URL.Query().Get("error"),
			Filter:          "",
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.Execute(w, pageData); err != nil {
			log.Println("Error executing register template:", err)
		}
	}
}

// LoginHandler выполняет вход пользователя.
// При GET перенаправляет на главную страницу, при POST аутентифицирует пользователя и создаёт сессию.
// Перенаправляет аутентифицированных пользователей на указанный URL или главную страницу.
func LoginHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if db == nil {
			log.Println("Database is not initialized.")
			writeError(w, http.StatusInternalServerError)
			return
		}

		isAuth, _, _ := IsAuthenticated(db, r)
		if isAuth {
			redirectURL := r.URL.Query().Get("redirect")
			if redirectURL == "" {
				redirectURL = "/"
			}
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}

		if r.Method == "GET" {
			log.Println("Redirecting GET /login to /.")
			http.Redirect(w, r, "/?message=Login+please", http.StatusSeeOther)
			return
		}

		if r.Method == "POST" {
			log.Println("Processing login attempt.")
			if err := r.ParseForm(); err != nil {
				log.Println("Error parsing form:", err)
				http.Redirect(w, r, "/?login_error=Bad request", http.StatusSeeOther)
				return
			}
			email := strings.TrimSpace(r.FormValue("email"))
			password := r.FormValue("password")

			if email == "" || password == "" {
				log.Println("Empty email or password.")
				http.Redirect(w, r, "/?login_error=Email and password are required", http.StatusSeeOther)
				return
			}

			userID, _, hashedPassword, role, err := database.GetUserByEmail(db, email)
			if err != nil {
				log.Printf("Error fetching user with email %s: %v", email, err)
				http.Redirect(w, r, "/?login_error=Invalid email or password", http.StatusSeeOther)
				return
			}

			err = bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
			if err != nil {
				log.Printf("Invalid password for email %s.", email)
				http.Redirect(w, r, "/?login_error=Invalid email or password", http.StatusSeeOther)
				return
			}

			err = database.DeleteUserSessions(db, userID)
			if err != nil {
				log.Println("Error deleting old sessions:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			sessionID := uuid.New().String()
			expiry := time.Now().Add(24 * time.Hour)
			err = database.CreateSession(db, sessionID, userID, role, expiry)
			if err != nil {
				log.Println("Error saving session:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}

			cookie := http.Cookie{
				Name:     "session_id",
				Value:    sessionID,
				Path:     "/",
				HttpOnly: true,
				MaxAge:   24 * 60 * 60,
				SameSite: http.SameSiteLaxMode,
			}
			http.SetCookie(w, &cookie)

			redirectURL := r.URL.Query().Get("redirect")
			if redirectURL == "" {
				redirectURL = "/"
			}
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
			return
		}

		log.Println("Method not allowed:", r.Method)
		w.Header().Set("Allow", "GET, POST")
		writeError(w, http.StatusMethodNotAllowed)
	}
}

// LogoutHandler выполняет выход пользователя.
// Удаляет сессию из базы данных и очищает cookie, затем перенаправляет на главную страницу.
func LogoutHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_id")
		if err == nil {
			err = database.DeleteSession(db, cookie.Value)
			if err != nil {
				log.Println("Error deleting session:", err)
			}

			http.SetCookie(w, &http.Cookie{
				Name:     "session_id",
				Value:    "",
				Expires:  time.Unix(0, 0),
				Path:     "/",
				HttpOnly: true,
			})
		}

		// ⬇️ ПЕРЕХОД НА СТИЛИЗОВАННУЮ 404 В КАТЕГОРИИ
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// ProfileHandler отображает профиль пользователя по его ID.
// Включает посты пользователя с категориями и комментариями.
func ProfileHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		isAuth, currentUserID, role := IsAuthenticated(db, r)
		var currentUsername string
		if isAuth {
			var err error
			currentUsername, err = database.GetUsernameByID(db, currentUserID)
			if err != nil {
				log.Println("Error fetching current username:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
		}

		userIDStr := r.URL.Query().Get("user_id")
		if userIDStr == "" {
			writeError(w, http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(userIDStr)
		if err != nil {
			log.Println("Invalid user_id format:", err)
			writeError(w, http.StatusBadRequest)
			return
		}

		profileUsername, createdAt, err := database.GetUserProfileData(db, userID)
		if err == sql.ErrNoRows {
			writeError(w, http.StatusBadRequest)
			return
		}
		if err != nil {
			log.Println("Error querying user:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		posts, err := database.GetUserPosts(db, userID)
		if err != nil {
			log.Println("Error querying user posts:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		for i := range posts {
			posts[i].Username = profileUsername
			categories, err := database.GetPostCategories(db, posts[i].ID)
			if err != nil {
				log.Println("Error querying categories for post:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			posts[i].Categories = categories
			if len(categories) > 0 {
				posts[i].Category = categories[0]
			}

			comments, err := database.GetCommentsByPostIDWithUserVote(db, currentUserID, posts[i].ID)
			if err != nil {
				log.Println("Error querying comments for post:", err)
				writeError(w, http.StatusInternalServerError)
				return
			}
			posts[i].Comments = comments
			posts[i].CreatedAtStr = createdAt.Format(time.DateOnly)
		}

		tmpl, err := template.ParseFiles("templates/profile.html")
		if err != nil {
			log.Println("Error parsing profile template:", err)
			writeError(w, http.StatusInternalServerError)
			return
		}

		pageData := models.PageData{
			IsAuthenticated:  isAuth,
			UserID:           currentUserID,
			Username:         currentUsername,
			Role:             role,
			Filter:           "",
			Posts:            posts,
			ProfileUsername:  profileUsername,
			ProfileCreatedAt: createdAt.Format(time.DateOnly),
		}
		if err := tmpl.Execute(w, pageData); err != nil {
			log.Println("Error executing profile template:", err)
			writeError(w, http.StatusInternalServerError)
		}
	}
}
