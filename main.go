package main

import (
	"database/sql"
	"forum/database"
	"log"
	"net/http"
)


// db хранит подключение к базе данных.
var db *sql.DB

// main инициализирует приложение и запускает сервер.
// Устанавливает соединение с базой данных, настраивает маршруты и слушает порт 8080.
func main() {
	var err error
	db, err = database.InitDB()
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Настраивает маршруты и возвращает обработчик HTTP-запросов.
	handler := setupRoutes(db)

	log.Println("Server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
