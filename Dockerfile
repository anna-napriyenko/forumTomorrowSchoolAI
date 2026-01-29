# Dockerfile описывает процесс сборки и запуска контейнера для приложения форума.
# Использует многоэтапную сборку: этап builder компилирует Go-приложение, финальный образ копирует бинарный файл и необходимые ресурсы.

FROM golang:1.23-alpine AS builder

# Устанавливает необходимые пакеты для сборки.
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
# Копирует файлы go.mod и go.sum, загружает зависимости.
COPY go.mod go.sum ./
RUN go mod download
# Копирует исходный код и компилирует приложение.
COPY . .
RUN CGO_ENABLED=1 go build -o server .

FROM alpine:latest
WORKDIR /app
# Копирует скомпилированный бинарный файл из этапа builder.
COPY --from=builder /app/server .
# Копирует шаблоны и статические файлы.
COPY templates ./templates
COPY static ./static
# Открывает порт 8080 для веб-сервера.
EXPOSE 8080
# Запускает приложение.
CMD ["./server"]

# Build image: docker image build -t forum:latest .
# Run container: docker container run -d -p 8080:8080 --name forum -v $(pwd)/forum.db:/app/forum.db forum:latest