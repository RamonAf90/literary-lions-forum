package models

import (
	"time"
)

// User represents a forum user with authentication and profile information.
type User struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// UserWithStats represents a user with their activity statistics for the members page.
type UserWithStats struct {
	ID                 int       `json:"id"`
	Username           string    `json:"username"`
	CreatedAt          time.Time `json:"created_at"`
	PostsCount         int       `json:"posts_count"`
	CommentsCount      int       `json:"comments_count"`
	TotalLikesReceived int       `json:"total_likes_received"`
}

// Session represents a user's active session for authentication.
type Session struct {
	ID        string    `json:"id"`
	UserID    int       `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// Category represents a forum category for organizing posts.
type Category struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

// Post represents a forum post with content, metadata, and engagement metrics.
type Post struct {
	ID                int       `json:"id"`
	Title             string    `json:"title"`
	Content           string    `json:"content"`
	UserID            int       `json:"user_id"`
	Username          string    `json:"username"`
	CategoryID        *int      `json:"category_id"`
	Category          *Category `json:"category"`
	ImageFilename     *string   `json:"image_filename"`      // New field
	ImageOriginalName *string   `json:"image_original_name"` // New field
	Likes             int       `json:"likes"`
	Dislikes          int       `json:"dislikes"`
	UserLiked         bool      `json:"user_liked"`
	UserDisliked      bool      `json:"user_disliked"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// Comment represents a user comment on a forum post.
type Comment struct {
	ID        int       `json:"id"`
	Content   string    `json:"content"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	PostID    int       `json:"post_id"`
	Likes     int       `json:"likes"`
	Dislikes  int       `json:"dislikes"`
	CreatedAt time.Time `json:"created_at"`
}

// PostLike represents a user's like/dislike vote on a post.
type PostLike struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	PostID    int       `json:"post_id"`
	IsLike    bool      `json:"is_like"`
	CreatedAt time.Time `json:"created_at"`
}

// CommentLike represents a user's like/dislike vote on a comment.
type CommentLike struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	CommentID int       `json:"comment_id"`
	IsLike    bool      `json:"is_like"`
	CreatedAt time.Time `json:"created_at"`
}

// UserStats represents aggregated user statistics for profile display.
type UserStats struct {
	PostsCount         int
	CommentsCount      int
	LikedPostsCount    int
	TotalLikesReceived int
}

// CommentWithPost represents a comment with its associated post title for display.
type CommentWithPost struct {
	ID        int
	Content   string
	UserID    int
	Username  string
	PostID    int
	PostTitle string
	Likes     int
	Dislikes  int
	CreatedAt time.Time
}
