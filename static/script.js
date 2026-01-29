// /static/script.js
let userRole; // Объявляем без значения, зададим его из HTML

document.addEventListener("DOMContentLoaded", () => {
    userRole = window.userRole || "";
    initCountdownTimer();
    createSnowfall();
});

function initCountdownTimer() {
    const countdownEl = document.getElementById("countdown-timer");
    const miniCountdownEl = document.getElementById("mini-countdown");
    if (!countdownEl && !miniCountdownEl) return;

    const targetDate = new Date("2026-01-01T00:00:00Z").getTime();

    const render = () => {
        const diff = Math.max(targetDate - Date.now(), 0);
        const days = Math.floor(diff / (1000 * 60 * 60 * 24));
        const hours = Math.floor((diff / (1000 * 60 * 60)) % 24);
        const minutes = Math.floor((diff / (1000 * 60)) % 60);
        const seconds = Math.floor((diff / 1000) % 60);
        const text = `${days}d • ${String(hours).padStart(2, "0")}h • ${String(minutes).padStart(2, "0")}m • ${String(seconds).padStart(2, "0")}s`;
        if (countdownEl) countdownEl.textContent = text;
        if (miniCountdownEl) miniCountdownEl.textContent = text;
    };

    render();
    setInterval(render, 1000);
}

function createSnowfall() {
    const snowflakes = 36;
    for (let i = 0; i < snowflakes; i++) {
        const flake = document.createElement("span");
        flake.className = "snowflake";
        flake.textContent = Math.random() > 0.5 ? "✺" : "✦";
        flake.style.left = `${Math.random() * 100}vw`;
        flake.style.animationDuration = `${8 + Math.random() * 10}s`;
        flake.style.fontSize = `${0.6 + Math.random() * 0.8}rem`;
        flake.style.animationDelay = `${Math.random() * -20}s`;
        document.body.appendChild(flake);
    }
}

function vote(postId, action) {
    fetch(`/${action}?post_id=${postId}`, { method: 'GET', credentials: 'same-origin' })
        .then(response => response.json())
        .then(data => {
            if (data.success) {
                document.getElementById(`likes-${postId}`).textContent = `Likes: ${data.likes}`;
                document.getElementById(`dislikes-${postId}`).textContent = `Dislikes: ${data.dislikes}`;
                const likeBtn = document.querySelector(`#votes-${postId} .vote-btn[data-action="like"]`);
                const dislikeBtn = document.querySelector(`#votes-${postId} .vote-btn[data-action="dislike"]`);
                likeBtn.classList.toggle('liked', data.user_vote === 1);
                dislikeBtn.classList.toggle('disliked', data.user_vote === -1);
            } else {
                alert(data.message);
            }
        })
        .catch(error => console.error('Error:', error));
}

function voteComment(commentId, action) {
    fetch(`/${action}?comment_id=${commentId}`, {
        method: 'POST', // Используем POST вместо GET
        credentials: 'same-origin'
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            document.getElementById(`comment-likes-${commentId}`).textContent = `Likes: ${data.likes}`;
            document.getElementById(`comment-dislikes-${commentId}`).textContent = `Dislikes: ${data.dislikes}`;
            const likeBtn = document.querySelector(`#comment-${commentId} .vote-btn[data-action="comment-like"]`);
            const dislikeBtn = document.querySelector(`#comment-${commentId} .vote-btn[data-action="comment-dislike"]`);
            likeBtn.classList.toggle('liked', data.user_vote === 1);
            dislikeBtn.classList.toggle('disliked', data.user_vote === -1);
        } else {
            alert(data.message);
        }
    })
    .catch(error => console.error('Error:', error));
}

function addComment(event, postId) {
    event.preventDefault();
    const form = event.target;
    const content = form.querySelector('textarea[name="content"]').value;
    const errorDiv = document.getElementById(`error-${postId}`);

    if (content.trim() === "") {
        errorDiv.textContent = "Comment content cannot be empty or contain only whitespace";
        errorDiv.style.display = "block";
        return;
    }
    if (content.trim().length < 3) {
        errorDiv.textContent = "Comment must be at least 3 characters long";
        errorDiv.style.display = "block";
        return;
    }
    if (content.trim().length > 500) {
        errorDiv.textContent = "Comment cannot be longer than 500 characters";
        errorDiv.style.display = "block";
        return;
    }

    errorDiv.style.display = "none";

    const formData = new URLSearchParams();
    formData.append("post_id", postId); // Используем postId напрямую
    formData.append("content", content);

    fetch("/comment", {
        method: "POST",
        body: formData,
        credentials: "same-origin",
        headers: {
            "Content-Type": "application/x-www-form-urlencoded"
        }
    })
    .then(response => response.json())
    .then(data => {
        if (data.success) {
            const commentsDiv = document.getElementById(`comments-${postId}`);
            const comment = document.createElement("div");
            comment.className = "comment";
            comment.id = `comment-${data.comment_id}`;
            comment.innerHTML = `
                <p>${data.content} by <a href="/profile?user_id=${data.user_id}">${data.username}</a> (${data.created_at})</p>
                <p id="comment-likes-${data.comment_id}">Likes: 0</p>
                <p id="comment-dislikes-${data.comment_id}">Dislikes: 0</p>
                <button onclick="voteComment(${data.comment_id}, 'comment-like')" class="vote-btn" data-action="comment-like">Like</button>
                <button onclick="voteComment(${data.comment_id}, 'comment-dislike')" class="vote-btn" data-action="comment-dislike">Dislike</button>
            `;
            comment.classList.add("fade-in");
            commentsDiv.appendChild(comment);
            form.reset();
        } else {
            errorDiv.textContent = data.message;
            errorDiv.style.display = "block";
        }
    })
    .catch(error => {
        console.error("Error adding comment:", error);
        errorDiv.textContent = "Failed to add comment";
        errorDiv.style.display = "block";
    });
}

function deleteComment(commentId) {
    if (confirm("Are you sure you want to delete this comment?")) {
        console.log("Deleting comment with ID:", commentId);

        fetch(`/delete-comment?comment_id=${commentId}`, {
            method: 'DELETE', // Используем DELETE
            credentials: 'same-origin',
        })
        .then(response => {
            console.log("Fetch response status:", response.status);
            return response.json();
        })
        .then(data => {
            console.log("Fetch response data:", data);
            if (data.success) {
                const notification = document.createElement('div');
                notification.className = 'notification success';
                notification.textContent = 'Comment deleted successfully';
                document.body.appendChild(notification);

                // Удаляем комментарий из DOM
                const commentElement = document.getElementById(`comment-${commentId}`);
                if (commentElement) {
                    console.log("Removing comment element from DOM:", `comment-${commentId}`);
                    commentElement.remove();
                } else {
                    console.warn(`Comment element with ID comment-${commentId} not found in DOM`);
                }

                setTimeout(() => notification.remove(), 3000);
            } else {
                alert(data.message);
            }
        })
        .catch(error => {
            console.error('Error in fetch:', error);
            alert('Failed to delete comment');
        });
    }
}

function validateCreatePostForm() {
    const select = document.querySelector('select[name="categories"]');
    const selectedOptions = select.selectedOptions;
    
    if (selectedOptions.length === 0) {
        alert("Please choose category");
        return false;
    }
    
    if (selectedOptions.length > 3) {
        alert("You can select up to 3 categories");
        return false;
    }
    
    return true;
}

function deletePost(postId) {
    if (confirm("Are you sure you want to delete this post?")) {
        console.log("Deleting post with ID:", postId);

        fetch(`/delete-post?post_id=${postId}`, {
            method: 'DELETE',
            credentials: 'same-origin'
        })
        .then(response => {
            console.log("Fetch response status:", response.status);
            return response.json();
        })
        .then(data => {
            console.log("Fetch response data:", data);
            if (data.success) {
	                // Показываем уведомление об успешном удалении
                const notification = document.createElement('div');
                notification.className = 'notification success';
                notification.textContent = 'Post deleted successfully';
                document.body.appendChild(notification);

                // Удаляем пост из DOM
                const postElement = document.getElementById(`post-${postId}`);
                if (postElement) {
                    console.log("Removing post element from DOM:", `post-${postId}`);
                    postElement.remove();
                } else {
                    console.warn(`Post element with ID post-${postId} not found in DOM`);
                }

                // Проверяем, остались ли посты
                const remainingPosts = document.querySelectorAll('.post-card');
                if (remainingPosts.length === 0) {
                    const postsSection = document.querySelector('.posts');
                    if (postsSection) {
                        console.log("No posts remaining, updating DOM");
                        postsSection.innerHTML = '<p class="no-posts">No posts available.</p>';
                    }
                }

                // Удаляем уведомление через 3 секунды
                setTimeout(() => notification.remove(), 3000);
            } else {
                alert(data.message);
            }
        })
        .catch(error => {
            console.error('Error in fetch:', error);
            alert('Failed to delete post');
        });
    }
}

function editPost(event, postId) {
    event.preventDefault(); // Предотвращаем стандартную отправку формы

    const form = document.getElementById('edit-post-form');
    const formData = new FormData(form);
    const csrfToken = document.getElementById('csrf_token').value;

    fetch(`/edit-post?post_id=${postId}`, {
        method: 'PUT',
        body: formData,
        credentials: 'same-origin',
        headers: {
            'X-CSRF-Token': csrfToken
        }
    })
    .then(response => {
        console.log("Fetch response status:", response.status);
        return response.json();
    })
    .then(data => {
        console.log("Fetch response data:", data);
        if (data.success) {
            const notification = document.createElement('div');
            notification.className = 'notification success';
            notification.textContent = 'Post updated successfully';
            document.body.appendChild(notification);
            setTimeout(() => {
                notification.remove();
                if (data.redirect) {
                    window.location.href = data.redirect; // Перенаправляем на /?filter=my
                }
            }, 2000);
        } else {
            alert(data.message);
        }
    })
    .catch(error => {
        console.error('Error in fetch:', error);
        alert('Failed to update post');
    });
}