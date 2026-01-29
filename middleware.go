// Package main содержит реализацию кастомного HTTP-обработчика для форума.
// Обрабатывает маршруты, логирует запросы, перехватывает паники и возвращает страницу 404 при отсутствии маршрута.

package main

import (
	"log"
	"net/http"
	"text/template"
)

// CustomHandler обрабатывает HTTP-запросы с перехватом паник и обработкой ошибок 404.
// Логирует запросы и ответы, рендерит шаблон 404 при отсутствии маршрута.
type CustomHandler struct {
	mux *http.ServeMux // Маршрутизатор для обработки запросов.
}

// ServeHTTP обрабатывает входящий HTTP-запрос.
// Перехватывает паники, логирует запросы и ответы, возвращает страницу 404, если маршрут не найден.
func (h *CustomHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if rec := recover(); rec != nil {
			log.Printf("Panic recovered: %v.", rec)
			http.Error(w, "Internal server error.", http.StatusInternalServerError)
		}
	}()

	log.Println("Incoming request:", r.Method, r.URL.Path)

	rr := &responseRecorder{ResponseWriter: w, statusCode: 0, written: false}
	h.mux.ServeHTTP(rr, r)

	log.Println("After mux: statusCode =", rr.statusCode, "written =", rr.written)

	if !rr.written {
		log.Println("Route not found:", r.URL.Path)
		if r.Method == http.MethodHead {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusNotFound)
		tmpl, err := template.ParseFiles("templates/404.html")
		if err != nil {
			log.Println("Error parsing 404 template:", err)
			http.Error(w, "Page not found.", http.StatusNotFound)
			return
		}
		err = tmpl.Execute(w, nil)
		if err != nil {
			log.Println("Error executing 404 template:", err)
		}
	}
}

// responseRecorder отслеживает статус ответа и факт записи.
// Используется для определения, был ли отправлен ответ маршрутизатором.
type responseRecorder struct {
	http.ResponseWriter
	statusCode int  // Код статуса ответа.
	written    bool // Флаг, указывающий, был ли записан ответ.
}

// WriteHeader записывает код статуса ответа.
// Устанавливает код и флаг written, если ответ ещё не был записан.
func (rec *responseRecorder) WriteHeader(code int) {
	if !rec.written {
		rec.statusCode = code
		rec.written = true
		rec.ResponseWriter.WriteHeader(code)
	}
}

// Write записывает данные в ответ.
// Устанавливает код 200 и флаг written, если ответ ещё не был записан.
func (rec *responseRecorder) Write(b []byte) (int, error) {
	if !rec.written {
		rec.statusCode = http.StatusOK
		rec.ResponseWriter.WriteHeader(http.StatusOK)
		rec.written = true
	}
	n, err := rec.ResponseWriter.Write(b)
	if err == nil && n > 0 {
		rec.written = true
	}
	return n, err
}
