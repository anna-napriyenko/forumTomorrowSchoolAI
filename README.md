🗨 **Forum Web Application**

A web forum built with Go featuring a modern interface and a full set of tools for user communication.

The project includes user registration and authentication, post creation, comments, a like system, administration features, and basic security mechanisms.

---

🚀 **Features**

### 👤 Users

* Registration (email, username, password)
* Authentication and session management
* User profile with avatar
* Password hashing using bcrypt

### 📝 Content

* Creating posts (text + images)
* Commenting on posts
* Post categories:

  * General
  * Technology
  * Science
  * Art
  * Sports
* Image uploads

### ❤️ Interaction

* Likes and dislikes for posts and comments
* Post filtering:

  * by category
  * created by the user
  * liked by the user
* Pagination

### 🛠 Administration

* Admin panel
* User management
* Deletion of posts and comments

### 🔒 Security

* CSRF protection
* Input validation
* Secure sessions using UUIDs

---

🛠 **Technologies Used**

* Backend: Go 1.23
* Database: SQLite
* Frontend: HTML, CSS, JavaScript
* Containerization: Docker
* Security: bcrypt, CSRF tokens

---

📦 **Installation and Launch**

### 🔧 Local Setup

1. Make sure Go 1.23+ is installed
2. Clone the repository:

   ```bash
   git clone <repository-url>
   cd forum
   ```
3. Run the application:

   ```bash
   go run main.go
   ```
4. Open in your browser:

   ```
   http://localhost:8080
   ```

---

🐳 **Running with Docker (recommended)**

### Quick Start

1. Make sure Docker is installed
2. Clone the repository:

   ```bash
   git clone <repository-url>
   cd forum
   ```
3. Run the application:

   ```bash
   ./run_docker.sh
   ```
4. Open in your browser:

   ```
   http://localhost:8080
   ```

---

⚙ **Manual Docker Setup**

1. Build the Docker image:

   ```bash
   docker build -t forum .
   ```
2. Run the container:

   ```bash
   docker run -d \
     -p 8080:8080 \
     -v $(pwd)/forum.db:/app/forum.db \
     --name forum forum
   ```

---

🧰 **Docker Container Management**

▶ Start:

```bash
./run_docker.sh
```

⏹ Stop:

```bash
./stop_docker.sh
```

📜 View logs:

```bash
docker logs forum
```

📦 Check status:

```bash
docker ps
```

---

📌 **Notes**

* The database is stored in the `forum.db` file
* Uploaded images are saved locally
* The project is suitable for educational and demonstration purposes

---

Project Developer — Anna Napriyenko
Nickname: anna-napriyenko

