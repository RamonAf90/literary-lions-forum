package database

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"literary-lions/models"
	"os"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// DB wraps the SQL database connection and provides methods for forum operations.
type DB struct {
	*sql.DB
}

// generateSessionID creates a random session ID using crypto/rand
func generateSessionID() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// hashPassword creates a SHA-256 hash with salt for password storage
func hashPassword(password string) (string, error) {
	// Generate a random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}

	// Create hash with salt
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	hashedPassword := h.Sum(nil)

	// Combine salt and hash for storage
	return hex.EncodeToString(salt) + ":" + hex.EncodeToString(hashedPassword), nil
}

// verifyPassword checks if password matches the stored hash
func verifyPassword(password, storedHash string) bool {
	// Split salt and hash
	parts := make([]string, 0, 2)
	colonIndex := -1
	for i, char := range storedHash {
		if char == ':' {
			colonIndex = i
			break
		}
	}
	if colonIndex == -1 {
		return false
	}

	parts = append(parts, storedHash[:colonIndex])
	parts = append(parts, storedHash[colonIndex+1:])

	if len(parts) != 2 {
		return false
	}

	// Decode salt
	salt, err := hex.DecodeString(parts[0])
	if err != nil {
		return false
	}

	// Decode stored hash
	storedPasswordHash, err := hex.DecodeString(parts[1])
	if err != nil {
		return false
	}

	// Create hash with provided password and stored salt
	h := sha256.New()
	h.Write([]byte(password))
	h.Write(salt)
	passwordHash := h.Sum(nil)

	// Compare hashes
	if len(passwordHash) != len(storedPasswordHash) {
		return false
	}

	for i := range passwordHash {
		if passwordHash[i] != storedPasswordHash[i] {
			return false
		}
	}

	return true
}

// NewDB creates a new database connection and returns a DB instance.
// dataSourceName should be the path to the SQLite database file.
func NewDB(dataSourceName string) (*DB, error) {
	db, err := sql.Open("sqlite3", dataSourceName)
	if err != nil {
		return nil, err
	}

	if err = db.Ping(); err != nil {
		return nil, err
	}

	return &DB{db}, nil
}

// InitSchema initializes the database schema by executing the SQL schema file.
// This should be called once when setting up a new database.
func (db *DB) InitSchema() error {
	schema, err := os.ReadFile("database/schema.sql")
	if err != nil {
		return fmt.Errorf("failed to read schema file: %v", err)
	}

	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("failed to execute schema: %v", err)
	}

	return nil
}

// CreateUser creates a new user account with the provided credentials.
// The password is automatically hashed before storage.
func (db *DB) CreateUser(email, username, password string) error {
	hashedPassword, err := hashPassword(password)
	if err != nil {
		return err
	}

	query := `INSERT INTO users (email, username, password_hash) VALUES (?, ?, ?)`
	_, err = db.Exec(query, email, username, hashedPassword)
	return err
}

// GetUserByEmail retrieves a user by their email address.
// Returns nil if no user is found with the given email.
func (db *DB) GetUserByEmail(email string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, username, password_hash, created_at FROM users WHERE email = ?`
	err := db.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByUsername retrieves a user by their username.
// Returns nil if no user is found with the given username.
func (db *DB) GetUserByUsername(username string) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, username, password_hash, created_at FROM users WHERE username = ?`
	err := db.QueryRow(query, username).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetUserByID retrieves a user by their ID.
// Returns nil if no user is found with the given ID.
func (db *DB) GetUserByID(id int) (*models.User, error) {
	user := &models.User{}
	query := `SELECT id, email, username, password_hash, created_at FROM users WHERE id = ?`
	err := db.QueryRow(query, id).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// GetAllUsers retrieves all users from the database for the members page.
// Returns users sorted by username alphabetically.
func (db *DB) GetAllUsers() ([]models.User, error) {
	var users []models.User
	query := `SELECT id, email, username, created_at FROM users ORDER BY username ASC`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Email, &user.Username, &user.CreatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// GetUsersWithStats retrieves all users with their post and comment counts.
// This provides more detailed information for the members page.
func (db *DB) GetUsersWithStats() ([]models.UserWithStats, error) {
	var users []models.UserWithStats
	query := `
		SELECT 
			u.id, 
			u.username, 
			u.created_at,
			COUNT(DISTINCT p.id) as posts_count,
			COUNT(DISTINCT c.id) as comments_count,
			COALESCE(SUM(p.likes), 0) as total_likes_received
		FROM users u
		LEFT JOIN posts p ON u.id = p.user_id
		LEFT JOIN comments c ON u.id = c.user_id
		GROUP BY u.id, u.username, u.created_at
		ORDER BY u.username ASC
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var user models.UserWithStats
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.CreatedAt,
			&user.PostsCount,
			&user.CommentsCount,
			&user.TotalLikesReceived,
		)
		if err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, rows.Err()
}

// CheckUserExists checks if a user with the given email or username already exists.
// Returns two booleans indicating if email and username exist respectively.
func (db *DB) CheckUserExists(email, username string) (bool, bool, error) {
	var emailExists, usernameExists bool

	err := db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE email = ?)", email).Scan(&emailExists)
	if err != nil {
		return false, false, err
	}

	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM users WHERE username = ?)", username).Scan(&usernameExists)
	if err != nil {
		return false, false, err
	}

	return emailExists, usernameExists, nil
}

// VerifyPassword checks if the provided password matches the stored hash.
// Returns true if the password is correct, false otherwise.
func (db *DB) VerifyPassword(password, storedHash string) bool {
	return verifyPassword(password, storedHash)
}

// CreateSession creates a new session for the given user ID.
// Returns the session ID string and any error encountered.
func (db *DB) CreateSession(userID int) (string, error) {
	sessionID, err := generateSessionID()
	if err != nil {
		return "", err
	}

	expiresAt := time.Now().Add(24 * time.Hour) // 24 hour session

	query := `INSERT INTO sessions (id, user_id, expires_at) VALUES (?, ?, ?)`
	_, err = db.Exec(query, sessionID, userID, expiresAt)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

// GetSession retrieves a session by its ID.
// Returns nil if no session is found or if it has expired.
func (db *DB) GetSession(sessionID string) (*models.Session, error) {
	session := &models.Session{}
	query := `SELECT id, user_id, expires_at, created_at FROM sessions WHERE id = ?`
	err := db.QueryRow(query, sessionID).Scan(&session.ID, &session.UserID, &session.ExpiresAt, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

// DeleteSession removes a session from the database.
func (db *DB) DeleteSession(sessionID string) error {
	query := `DELETE FROM sessions WHERE id = ?`
	_, err := db.Exec(query, sessionID)
	return err
}

// CleanExpiredSessions removes all expired sessions from the database.
// This should be called periodically to maintain database performance.
func (db *DB) CleanExpiredSessions() error {
	query := `DELETE FROM sessions WHERE expires_at < ?`
	_, err := db.Exec(query, time.Now())
	return err
}

// GetCategories retrieves all forum categories.
// Returns an empty slice if no categories exist.
func (db *DB) GetCategories() ([]models.Category, error) {
	var categories []models.Category
	query := `SELECT id, name, description, created_at FROM categories ORDER BY name`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var category models.Category
		err := rows.Scan(&category.ID, &category.Name, &category.Description, &category.CreatedAt)
		if err != nil {
			return nil, err
		}
		categories = append(categories, category)
	}

	return categories, rows.Err()
}

// GetCategoryByID retrieves a category by its ID.
// Returns nil if no category is found with the given ID.
func (db *DB) GetCategoryByID(id int) (*models.Category, error) {
	category := &models.Category{}
	query := `SELECT id, name, description, created_at FROM categories WHERE id = ?`
	err := db.QueryRow(query, id).Scan(&category.ID, &category.Name, &category.Description, &category.CreatedAt)
	if err != nil {
		return nil, err
	}
	return category, nil
}

// CreatePost creates a new forum post with the provided content and metadata.
// imageFilename and imageOriginalName are optional and can be nil.
func (db *DB) CreatePost(title, content string, userID int, categoryID *int, imageFilename, imageOriginalName *string) error {
	query := `INSERT INTO posts (title, content, user_id, category_id, image_filename, image_original_name, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	now := time.Now()
	_, err := db.Exec(query, title, content, userID, categoryID, imageFilename, imageOriginalName, now, now)
	return err
}

// GetPosts retrieves posts with optional filtering by category, user, or liked by user.
// All filter parameters can be nil to disable filtering.
func (db *DB) GetPosts(categoryID *int, userID *int, likedByUserID *int) ([]models.Post, error) {
	var whereConditions []string
	var args []interface{}
	argIndex := 1

	if categoryID != nil {
		whereConditions = append(whereConditions, "p.category_id = ?")
		args = append(args, *categoryID)
		argIndex++
	}

	if userID != nil {
		whereConditions = append(whereConditions, "p.user_id = ?")
		args = append(args, *userID)
		argIndex++
	}

	if likedByUserID != nil {
		whereConditions = append(whereConditions, "p.id IN (SELECT post_id FROM post_likes WHERE user_id = ? AND is_like = 1)")
		args = append(args, *likedByUserID)
		argIndex++
	}

	whereClause := ""
	if len(whereConditions) > 0 {
		whereClause = "WHERE " + whereConditions[0]
		for i := 1; i < len(whereConditions); i++ {
			whereClause += " AND " + whereConditions[i]
		}
	}

	query := fmt.Sprintf(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.category_id, 
		       c.name, c.description, p.image_filename, p.image_original_name,
		       p.likes, p.dislikes, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		%s
		ORDER BY p.created_at DESC
	`, whereClause)

	var posts []models.Post
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post models.Post
		var categoryName, categoryDescription sql.NullString
		var imageFilename, imageOriginalName sql.NullString
		err := rows.Scan(
			&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username,
			&post.CategoryID, &categoryName, &categoryDescription,
			&imageFilename, &imageOriginalName,
			&post.Likes, &post.Dislikes, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if post.CategoryID != nil && categoryName.Valid {
			post.Category = &models.Category{
				ID:          *post.CategoryID,
				Name:        categoryName.String,
				Description: categoryDescription.String,
			}
		}

		if imageFilename.Valid {
			post.ImageFilename = &imageFilename.String
		}
		if imageOriginalName.Valid {
			post.ImageOriginalName = &imageOriginalName.String
		}

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// GetPostByID retrieves a specific post by its ID.
// Returns nil if no post is found with the given ID.
func (db *DB) GetPostByID(postID int) (*models.Post, error) {
	query := `
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.category_id, 
		       c.name, c.description, p.image_filename, p.image_original_name,
		       p.likes, p.dislikes, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		WHERE p.id = ?
	`

	var post models.Post
	var categoryName, categoryDescription sql.NullString
	var imageFilename, imageOriginalName sql.NullString
	err := db.QueryRow(query, postID).Scan(
		&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username,
		&post.CategoryID, &categoryName, &categoryDescription,
		&imageFilename, &imageOriginalName,
		&post.Likes, &post.Dislikes, &post.CreatedAt, &post.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	if post.CategoryID != nil && categoryName.Valid {
		post.Category = &models.Category{
			ID:          *post.CategoryID,
			Name:        categoryName.String,
			Description: categoryDescription.String,
		}
	}

	if imageFilename.Valid {
		post.ImageFilename = &imageFilename.String
	}
	if imageOriginalName.Valid {
		post.ImageOriginalName = &imageOriginalName.String
	}

	return &post, nil
}

// CreateComment creates a new comment on a post.
func (db *DB) CreateComment(content string, userID, postID int) error {
	query := `INSERT INTO comments (content, user_id, post_id, created_at) VALUES (?, ?, ?, ?)`
	_, err := db.Exec(query, content, userID, postID, time.Now())
	return err
}

// GetCommentsByPostID retrieves all comments for a specific post.
// Returns an empty slice if no comments exist for the post.
func (db *DB) GetCommentsByPostID(postID int) ([]models.Comment, error) {
	var comments []models.Comment
	query := `
		SELECT c.id, c.content, c.user_id, u.username, c.post_id, c.likes, c.dislikes, c.created_at
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.post_id = ?
		ORDER BY c.created_at ASC
	`
	rows, err := db.Query(query, postID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var comment models.Comment
		err := rows.Scan(
			&comment.ID, &comment.Content, &comment.UserID, &comment.Username,
			&comment.PostID, &comment.Likes, &comment.Dislikes, &comment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	return comments, rows.Err()
}

// GetCommentByID retrieves a specific comment by its ID.
// Returns nil if no comment is found with the given ID.
func (db *DB) GetCommentByID(commentID int) (*models.Comment, error) {
	comment := &models.Comment{}
	query := `
		SELECT c.id, c.content, c.user_id, u.username, c.post_id, c.likes, c.dislikes, c.created_at
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		WHERE c.id = ?
	`
	err := db.QueryRow(query, commentID).Scan(
		&comment.ID, &comment.Content, &comment.UserID, &comment.Username,
		&comment.PostID, &comment.Likes, &comment.Dislikes, &comment.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return comment, nil
}

// LikePost handles a user's like/dislike vote on a post.
// If the user has already voted, their vote is updated.
// If isLike is nil, the vote is removed.
func (db *DB) LikePost(userID, postID int, isLike *bool) error {
	// Check if user already voted
	var existingIsLike bool
	err := db.QueryRow("SELECT is_like FROM post_likes WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&existingIsLike)
	if err == sql.ErrNoRows {
		if isLike == nil {
			// No vote to remove
			return nil
		}
		// New vote
		_, err = db.Exec("INSERT INTO post_likes (user_id, post_id, is_like, created_at) VALUES (?, ?, ?, ?)", userID, postID, *isLike, time.Now())
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if isLike == nil {
			// Remove existing vote
			_, err = db.Exec("DELETE FROM post_likes WHERE user_id = ? AND post_id = ?", userID, postID)
			if err != nil {
				return err
			}
		} else {
			// Update existing vote
			_, err = db.Exec("UPDATE post_likes SET is_like = ? WHERE user_id = ? AND post_id = ?", *isLike, userID, postID)
			if err != nil {
				return err
			}
		}
	}

	// Update post like/dislike counts
	likeCountQuery := "SELECT COUNT(*) FROM post_likes WHERE post_id = ? AND is_like = 1"
	dislikeCountQuery := "SELECT COUNT(*) FROM post_likes WHERE post_id = ? AND is_like = 0"

	var likes, dislikes int
	db.QueryRow(likeCountQuery, postID).Scan(&likes)
	db.QueryRow(dislikeCountQuery, postID).Scan(&dislikes)

	_, err = db.Exec("UPDATE posts SET likes = ?, dislikes = ? WHERE id = ?", likes, dislikes, postID)
	return err
}

// LikeComment handles a user's like/dislike vote on a comment.
// If the user has already voted, their vote is updated.
// If isLike is nil, the vote is removed.
func (db *DB) LikeComment(userID, commentID int, isLike *bool) error {
	// Check if user already voted
	var existingIsLike bool
	err := db.QueryRow("SELECT is_like FROM comment_likes WHERE user_id = ? AND comment_id = ?", userID, commentID).Scan(&existingIsLike)
	if err == sql.ErrNoRows {
		if isLike == nil {
			// No vote to remove
			return nil
		}
		// New vote
		_, err = db.Exec("INSERT INTO comment_likes (user_id, comment_id, is_like, created_at) VALUES (?, ?, ?, ?)", userID, commentID, *isLike, time.Now())
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else {
		if isLike == nil {
			// Remove existing vote
			_, err = db.Exec("DELETE FROM comment_likes WHERE user_id = ? AND comment_id = ?", userID, commentID)
			if err != nil {
				return err
			}
		} else {
			// Update existing vote
			_, err = db.Exec("UPDATE comment_likes SET is_like = ? WHERE user_id = ? AND comment_id = ?", *isLike, userID, commentID)
			if err != nil {
				return err
			}
		}
	}

	// Update comment like/dislike counts
	likeCountQuery := "SELECT COUNT(*) FROM comment_likes WHERE comment_id = ? AND is_like = 1"
	dislikeCountQuery := "SELECT COUNT(*) FROM comment_likes WHERE comment_id = ? AND is_like = 0"

	var likes, dislikes int
	db.QueryRow(likeCountQuery, commentID).Scan(&likes)
	db.QueryRow(dislikeCountQuery, commentID).Scan(&dislikes)

	_, err = db.Exec("UPDATE comments SET likes = ?, dislikes = ? WHERE id = ?", likes, dislikes, commentID)
	return err
}

// GetUserVoteForPost retrieves a user's vote on a specific post.
// Returns nil if the user hasn't voted, true for like, false for dislike.
func (db *DB) GetUserVoteForPost(userID, postID int) (*bool, error) {
	var isLike bool
	err := db.QueryRow("SELECT is_like FROM post_likes WHERE user_id = ? AND post_id = ?", userID, postID).Scan(&isLike)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &isLike, nil
}

// GetUserVoteForComment retrieves a user's vote on a specific comment.
// Returns nil if the user hasn't voted, true for like, false for dislike.
func (db *DB) GetUserVoteForComment(userID, commentID int) (*bool, error) {
	var isLike bool
	err := db.QueryRow("SELECT is_like FROM comment_likes WHERE user_id = ? AND comment_id = ?", userID, commentID).Scan(&isLike)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &isLike, nil
}

// SearchPosts searches for posts containing the query string in title or content.
// Results can be filtered by category ID (optional).
func (db *DB) SearchPosts(query string, categoryID *int) ([]models.Post, error) {
	var args []interface{}
	searchCondition := "WHERE (p.title LIKE ? OR p.content LIKE ?)"
	args = append(args, "%"+query+"%", "%"+query+"%")

	if categoryID != nil {
		searchCondition += " AND p.category_id = ?"
		args = append(args, *categoryID)
	}

	sqlQuery := fmt.Sprintf(`
		SELECT p.id, p.title, p.content, p.user_id, u.username, p.category_id, 
		       c.name, c.description, p.image_filename, p.image_original_name,
		       p.likes, p.dislikes, p.created_at, p.updated_at
		FROM posts p
		LEFT JOIN users u ON p.user_id = u.id
		LEFT JOIN categories c ON p.category_id = c.id
		%s
		ORDER BY p.created_at DESC
	`, searchCondition)

	var posts []models.Post
	rows, err := db.Query(sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var post models.Post
		var categoryName, categoryDescription string
		var imageFilename, imageOriginalName sql.NullString
		err := rows.Scan(
			&post.ID, &post.Title, &post.Content, &post.UserID, &post.Username,
			&post.CategoryID, &categoryName, &categoryDescription,
			&imageFilename, &imageOriginalName,
			&post.Likes, &post.Dislikes, &post.CreatedAt, &post.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		if post.CategoryID != nil {
			post.Category = &models.Category{
				ID:          *post.CategoryID,
				Name:        categoryName,
				Description: categoryDescription,
			}
		}

		if imageFilename.Valid {
			post.ImageFilename = &imageFilename.String
		}
		if imageOriginalName.Valid {
			post.ImageOriginalName = &imageOriginalName.String
		}

		posts = append(posts, post)
	}

	return posts, rows.Err()
}

// GetUserStats retrieves aggregated statistics for a user.
// Returns nil if no user is found with the given ID.
func (db *DB) GetUserStats(userID int) (*models.UserStats, error) {
	stats := &models.UserStats{}

	// Count posts
	err := db.QueryRow("SELECT COUNT(*) FROM posts WHERE user_id = ?", userID).Scan(&stats.PostsCount)
	if err != nil {
		return nil, err
	}

	// Count comments
	err = db.QueryRow("SELECT COUNT(*) FROM comments WHERE user_id = ?", userID).Scan(&stats.CommentsCount)
	if err != nil {
		return nil, err
	}

	// Count liked posts
	err = db.QueryRow("SELECT COUNT(*) FROM post_likes WHERE user_id = ? AND is_like = 1", userID).Scan(&stats.LikedPostsCount)
	if err != nil {
		return nil, err
	}

	// Count total likes received on posts
	err = db.QueryRow("SELECT COALESCE(SUM(likes), 0) FROM posts WHERE user_id = ?", userID).Scan(&stats.TotalLikesReceived)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// GetUserPosts retrieves all posts by a specific user.
// Returns an empty slice if the user has no posts.
func (db *DB) GetUserPosts(userID int) ([]models.Post, error) {
	return db.GetPosts(nil, &userID, nil)
}

// GetUserLikedPosts retrieves all posts liked by a specific user.
// Returns an empty slice if the user has no liked posts.
func (db *DB) GetUserLikedPosts(userID int) ([]models.Post, error) {
	return db.GetPosts(nil, nil, &userID)
}

// GetUserComments retrieves all comments by a specific user with post titles.
// Returns an empty slice if the user has no comments.
func (db *DB) GetUserComments(userID int) ([]models.CommentWithPost, error) {
	var comments []models.CommentWithPost
	query := `
		SELECT c.id, c.content, c.user_id, u.username, c.post_id, p.title, c.likes, c.dislikes, c.created_at
		FROM comments c
		LEFT JOIN users u ON c.user_id = u.id
		LEFT JOIN posts p ON c.post_id = p.id
		WHERE c.user_id = ?
		ORDER BY c.created_at DESC
	`
	rows, err := db.Query(query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var comment models.CommentWithPost
		err := rows.Scan(
			&comment.ID, &comment.Content, &comment.UserID, &comment.Username,
			&comment.PostID, &comment.PostTitle, &comment.Likes, &comment.Dislikes, &comment.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		comments = append(comments, comment)
	}

	return comments, rows.Err()
}

// DeletePost removes a post and all its associated data from the database.
// This includes comments, likes, and any associated images.
func (db *DB) DeletePost(postID int) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete comment likes for comments on this post
	_, err = tx.Exec("DELETE FROM comment_likes WHERE comment_id IN (SELECT id FROM comments WHERE post_id = ?)", postID)
	if err != nil {
		return err
	}

	// Delete comments on this post
	_, err = tx.Exec("DELETE FROM comments WHERE post_id = ?", postID)
	if err != nil {
		return err
	}

	// Delete post likes
	_, err = tx.Exec("DELETE FROM post_likes WHERE post_id = ?", postID)
	if err != nil {
		return err
	}

	// Delete the post
	_, err = tx.Exec("DELETE FROM posts WHERE id = ?", postID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// DeleteComment removes a comment and its associated likes from the database.
func (db *DB) DeleteComment(commentID int) error {
	// Start a transaction
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Delete comment likes
	_, err = tx.Exec("DELETE FROM comment_likes WHERE comment_id = ?", commentID)
	if err != nil {
		return err
	}

	// Delete the comment
	_, err = tx.Exec("DELETE FROM comments WHERE id = ?", commentID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// Close closes the database connection.
// This should be called when the application shuts down.
func (db *DB) Close() error {
	// Clean up expired sessions before closing
	db.CleanExpiredSessions()
	return db.DB.Close()
}
