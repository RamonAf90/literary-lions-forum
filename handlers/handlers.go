package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"literary-lions/database"
	"literary-lions/models"
	"literary-lions/utils"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)


// TemplatePost represents a post with enriched data for template rendering.
// This includes user vote information and formatted data for display.
type TemplatePost struct {
	ID                int
	Title             string
	Content           string
	UserID            int
	Username          string
	CategoryID        *int
	Category          *models.Category
	ImageFilename     *string
	ImageOriginalName *string
	Likes             int
	Dislikes          int
	UserLiked         bool
	UserDisliked      bool
	HasVote           bool
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// TemplateComment represents a comment with enriched data for template rendering.
// This includes user vote information and formatted data for display.
type TemplateComment struct {
	ID           int
	Content      string
	UserID       int
	Username     string
	PostID       int
	Likes        int
	Dislikes     int
	UserLiked    bool
	UserDisliked bool
	HasVote      bool
	CreatedAt    time.Time
}

// Handler manages HTTP request handling and template rendering for the forum.
type Handler struct {
	DB        *database.DB
	Templates *template.Template
}


// NewHandler creates a new Handler instance with the provided database connection.
// It initializes the template system by parsing all HTML templates with custom functions.
func NewHandler(db *database.DB) *Handler {
	// Define custom template functions
	funcMap := template.FuncMap{
		"atoi": func(s string) int {
			i, _ := strconv.Atoi(s)
			return i
		},
		"printf": fmt.Sprintf,
		"slice": func(s string, start, end int) string {
			if start < 0 {
				start = 0
			}
			if end > len(s) {
				end = len(s)
			}
			if start >= end {
				return ""
			}
			return s[start:end]
		},
		"len": func(v interface{}) int {
			switch val := v.(type) {
			case []models.Post:
				return len(val)
			case []models.Comment:
				return len(val)
			case []TemplateComment:
				return len(val)
			case []models.CommentWithPost:
				return len(val)
			case []models.UserWithStats:
				return len(val)
			case string:
				return len(val)
			default:
				return 0
			}
		},
		"gt": func(a, b int) bool {
			return a > b
		},
		"eq": func(a, b interface{}) bool {
			return a == b
		},
		"ne": func(a, b interface{}) bool {
			return a != b
		},
	}

	// Parse templates with custom functions
	tmpl := template.New("").Funcs(funcMap)
	tmpl = template.Must(tmpl.ParseGlob("templates/*.html"))

	return &Handler{
		DB:        db,
		Templates: tmpl,
	}
}

// render is a helper function that renders a template with the given data.
// It handles template errors gracefully and returns appropriate HTTP errors.
func (h *Handler) render(w http.ResponseWriter, tmpl string, data interface{}) {
	if err := h.Templates.ExecuteTemplate(w, tmpl, data); err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// getCurrentUser retrieves the currently authenticated user from the session.
// Returns nil if no valid session exists or if the session has expired.
func (h *Handler) getCurrentUser(r *http.Request) *models.User {
	cookie, err := r.Cookie("session_id")
	if err != nil {
		return nil
	}

	session, err := h.DB.GetSession(cookie.Value)
	if err != nil || session.ExpiresAt.Before(time.Now()) {
		if err == nil {
			h.DB.DeleteSession(session.ID)
		}
		return nil
	}

	user, _ := h.DB.GetUserByID(session.UserID)
	return user
}

// createSession creates a new session for the given user and sets the session cookie.
// The session is valid for 24 hours.
func (h *Handler) createSession(w http.ResponseWriter, userID int) error {
	sessionID, err := h.DB.CreateSession(userID)
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
	})
	return nil
}

// enrichPosts converts database Post models to TemplatePost models with vote information.
// This adds user-specific vote data to posts for template rendering.
func (h *Handler) enrichPosts(posts []models.Post, user *models.User) []TemplatePost {
	templatePosts := make([]TemplatePost, len(posts))
	for i, post := range posts {
		templatePosts[i] = TemplatePost{
			ID: post.ID, Title: post.Title, Content: post.Content,
			UserID: post.UserID, Username: post.Username,
			CategoryID: post.CategoryID, Category: post.Category,
			ImageFilename: post.ImageFilename, ImageOriginalName: post.ImageOriginalName,
			Likes: post.Likes, Dislikes: post.Dislikes,
			CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt,
		}
		if user != nil {
			if vote, err := h.DB.GetUserVoteForPost(user.ID, post.ID); err == nil && vote != nil {
				templatePosts[i].HasVote, templatePosts[i].UserLiked, templatePosts[i].UserDisliked = true, *vote, !*vote
			}
		}
	}
	return templatePosts
}

// enrichComments converts database Comment models to TemplateComment models with vote information.
// This adds user-specific vote data to comments for template rendering.
func (h *Handler) enrichComments(comments []models.Comment, user *models.User) []TemplateComment {
	templateComments := make([]TemplateComment, len(comments))
	for i, comment := range comments {
		templateComments[i] = TemplateComment{
			ID: comment.ID, Content: comment.Content, UserID: comment.UserID,
			Username: comment.Username, PostID: comment.PostID,
			Likes: comment.Likes, Dislikes: comment.Dislikes, CreatedAt: comment.CreatedAt,
		}
		if user != nil {
			if vote, err := h.DB.GetUserVoteForComment(user.ID, comment.ID); err == nil && vote != nil {
				templateComments[i].HasVote, templateComments[i].UserLiked, templateComments[i].UserDisliked = true, *vote, !*vote
			}
		}
	}
	return templateComments
}

// AuthData represents data passed to authentication-related templates.
type AuthData struct {
	Error, Page string
	User        *models.User
}

// authError renders an authentication error page with the given message.
func (h *Handler) authError(w http.ResponseWriter, page, msg string) {
	h.render(w, "base.html", AuthData{Error: msg, Page: page})
}

// parseFilters extracts and validates filter parameters from the request URL.
// Returns pointers to category ID, user ID, and liked by user ID filters.
func (h *Handler) parseFilters(r *http.Request, user *models.User) (*int, *int, *int) {
	var categoryID, userID, likedByUserID *int

	if cat := r.URL.Query().Get("category"); cat != "" {
		if id, err := strconv.Atoi(cat); err == nil {
			categoryID = &id
		}
	}
	if r.URL.Query().Get("user") == "me" && user != nil {
		userID = &user.ID
	}
	if r.URL.Query().Get("liked") == "me" && user != nil {
		likedByUserID = &user.ID
	}
	return categoryID, userID, likedByUserID
}

// Home handles the main forum page displaying posts with optional filtering.
// Supports filtering by category, user, and liked posts.
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		h.NotFound(w, r)
		return
	}

	user := h.getCurrentUser(r)
	categoryID, userID, likedByUserID := h.parseFilters(r, user)

	posts, err := h.DB.GetPosts(categoryID, userID, likedByUserID)
	if err != nil {
		h.InternalServerError(w, r)
		return
	}

	categories, err := h.DB.GetCategories()
	if err != nil {
		h.InternalServerError(w, r)
		return
	}

	h.render(w, "base.html", struct {
		Posts      []TemplatePost
		Categories []models.Category
		User       *models.User
		Page       string
		Filter     struct{ Category, User, Liked, Deleted string }
	}{
		Posts: h.enrichPosts(posts, user), Categories: categories, User: user, Page: "home",
		Filter: struct{ Category, User, Liked, Deleted string }{
			Category: r.URL.Query().Get("category"),
			User:     r.URL.Query().Get("user"),
			Liked:    r.URL.Query().Get("liked"),
			Deleted:  r.URL.Query().Get("deleted"),
		},
	})
}

// RegisterGet handles the user registration page display.
func (h *Handler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	if h.getCurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "base.html", AuthData{Page: "register"})
}

// RegisterPost handles user registration form submission.
// Validates input, checks for existing users, and creates the account.
func (h *Handler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")


	if email == "" || username == "" || password == "" {
		h.authError(w, "register", "All fields are required")
		return
	}

	emailExists, usernameExists, err := h.DB.CheckUserExists(email, username)
	if err != nil {
		h.InternalServerError(w, r)
		return
	}
	if emailExists {
		h.authError(w, "register", "Email already exists")
		return
	}
	if usernameExists {
		h.authError(w, "register", "Username already exists")
		return
	}

	if err := h.DB.CreateUser(email, username, password); err != nil {
		h.InternalServerError(w, r)
		return
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// LoginGet handles the login page display.
func (h *Handler) LoginGet(w http.ResponseWriter, r *http.Request) {
	if h.getCurrentUser(r) != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.render(w, "base.html", AuthData{Page: "login"})
}

// LoginPost handles user login form submission.
// Validates credentials and creates a session for the user.
func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := r.FormValue("password")

	if email == "" || password == "" {
		h.authError(w, "login", "Email and password are required")
		return
	}

	user, err := h.DB.GetUserByEmail(email)
	if err != nil || !h.DB.VerifyPassword(password, user.PasswordHash) {
		h.authError(w, "login", "Invalid email or password")
		return
	}

	if err := h.createSession(w, user.ID); err != nil {
		h.InternalServerError(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// Logout handles user logout by clearing the session cookie.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if cookie, err := r.Cookie("session_id"); err == nil {
		h.DB.DeleteSession(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name: "session_id", Value: "", Path: "/",
		Expires: time.Now().Add(-time.Hour), HttpOnly: true,
	})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// CreatePostGet handles the post creation page display.
// Requires user authentication.
func (h *Handler) CreatePostGet(w http.ResponseWriter, r *http.Request) {
	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	categories, err := h.DB.GetCategories()
	if err != nil {
		h.InternalServerError(w, r)
		return
	}

	h.render(w, "base.html", struct {
		Categories  []models.Category
		User        *models.User
		Error, Page string
	}{Categories: categories, User: user, Page: "create-post"})
}

// CreatePostPost handles post creation form submission.
// Validates input, processes image uploads, and creates the post.
func (h *Handler) CreatePostPost(w http.ResponseWriter, r *http.Request) {
	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.renderPostError(w, user, "Failed to parse form data")
		return
	}

	title, content := strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("content"))
	if title == "" || content == "" {
		h.renderPostError(w, user, "Title and content are required")
		return
	}

	// Validate category is selected
	catStr := strings.TrimSpace(r.FormValue("category"))
	if catStr == "" {
		h.renderPostError(w, user, "Please select a category for your post")
		return
	}

	var categoryID *int
	if id, err := strconv.Atoi(catStr); err == nil {
		categoryID = &id
	} else {
		h.renderPostError(w, user, "Invalid category selected")
		return
	}

	// Verify the category exists
	if _, err := h.DB.GetCategoryByID(*categoryID); err != nil {
		h.renderPostError(w, user, "Selected category does not exist")
		return
	}

	// Handle image upload
	var imageFilename, imageOriginalName *string
	if file, header, err := r.FormFile("image"); err == nil {
		defer file.Close()
		if filename, err := utils.SaveUploadedImage(file, header); err != nil {
			h.renderPostError(w, user, fmt.Sprintf("Image upload failed: %v", err))
			return
		} else {
			imageFilename, imageOriginalName = &filename, &header.Filename
		}
	} else if err != http.ErrMissingFile {
		h.renderPostError(w, user, "Failed to process image upload")
		return
	}

	if err := h.DB.CreatePost(title, content, user.ID, categoryID, imageFilename, imageOriginalName); err != nil {
		if imageFilename != nil {
			utils.DeleteImage(*imageFilename)
		}
		h.InternalServerError(w, r)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// renderPostError renders the create post page with an error message.
func (h *Handler) renderPostError(w http.ResponseWriter, user *models.User, msg string) {
	categories, _ := h.DB.GetCategories()
	h.render(w, "base.html", struct {
		Categories  []models.Category
		User        *models.User
		Error, Page string
	}{Categories: categories, User: user, Error: msg, Page: "create-post"})
}

// PostDetail handles individual post page display with comments.
// Supports URL parameters for post ID extraction.
func (h *Handler) PostDetail(w http.ResponseWriter, r *http.Request) {
	postID, err := strconv.Atoi(r.URL.Path[len("/post/"):])
	if err != nil {
		h.NotFound(w, r)
		return
	}

	post, err := h.DB.GetPostByID(postID)
	if err != nil {
		if err == sql.ErrNoRows {
			h.NotFound(w, r)
		} else {
			h.InternalServerError(w, r)
		}
		return
	}

	comments, err := h.DB.GetCommentsByPostID(postID)
	if err != nil {
		h.InternalServerError(w, r)
		return
	}

	user := h.getCurrentUser(r)
	templatePost := &TemplatePost{
		ID: post.ID, Title: post.Title, Content: post.Content,
		UserID: post.UserID, Username: post.Username,
		CategoryID: post.CategoryID, Category: post.Category,
		ImageFilename: post.ImageFilename, ImageOriginalName: post.ImageOriginalName,
		Likes: post.Likes, Dislikes: post.Dislikes,
		CreatedAt: post.CreatedAt, UpdatedAt: post.UpdatedAt,
	}

	if user != nil {
		if vote, err := h.DB.GetUserVoteForPost(user.ID, post.ID); err == nil && vote != nil {
			templatePost.HasVote, templatePost.UserLiked, templatePost.UserDisliked = true, *vote, !*vote
		}
	}

	h.render(w, "base.html", struct {
		*TemplatePost
		Comments       []TemplateComment
		User           *models.User
		Page           string
		CommentDeleted string
	}{templatePost, h.enrichComments(comments, user), user, "post-detail", r.URL.Query().Get("deleted")})
}

// CreateComment handles comment creation form submission.
// Requires user authentication and valid post ID.
func (h *Handler) CreateComment(w http.ResponseWriter, r *http.Request) {
	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postIDStr, content := r.FormValue("post_id"), strings.TrimSpace(r.FormValue("content"))
	postID, err := strconv.Atoi(postIDStr)
	if err != nil || content == "" {
		http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
		return
	}

	if err := h.DB.CreateComment(content, user.ID, postID); err != nil {
		h.InternalServerError(w, r)
		return
	}
	http.Redirect(w, r, "/post/"+postIDStr, http.StatusSeeOther)
}

// handleLike is a generic handler for like/dislike functionality.
// It works for both posts and comments using the provided like function.
func (h *Handler) handleLike(w http.ResponseWriter, r *http.Request, idParam string, likeFunc func(int, int, *bool) error) {
	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	id, err := strconv.Atoi(r.FormValue(idParam))
	if err != nil {
		h.BadRequest(w, r)
		return
	}

	action := r.FormValue("action")
	var isLike *bool
	switch action {
	case "like":
		val := true
		isLike = &val
	case "dislike":
		val := false
		isLike = &val
	case "remove":
		isLike = nil
	default:
		h.BadRequest(w, r)
		return
	}

	if err := likeFunc(user.ID, id, isLike); err != nil {
		h.InternalServerError(w, r)
		return
	}

	if referer := r.Header.Get("Referer"); referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
	} else {
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

// LikePost handles post like/dislike functionality.
func (h *Handler) LikePost(w http.ResponseWriter, r *http.Request) {
	h.handleLike(w, r, "post_id", h.DB.LikePost)
}

// LikeComment handles comment like/dislike functionality.
func (h *Handler) LikeComment(w http.ResponseWriter, r *http.Request) {
	h.handleLike(w, r, "comment_id", h.DB.LikeComment)
}

// SearchGet handles the search page display.
func (h *Handler) SearchGet(w http.ResponseWriter, r *http.Request) {
	categories, _ := h.DB.GetCategories()
	h.render(w, "base.html", struct {
		Page, SearchQuery, SelectedCategory string
		User                                *models.User
		Categories                          []models.Category
		Posts                               []models.Post
		ResultsCount                        int
	}{Page: "search", User: h.getCurrentUser(r), Categories: categories})
}

// SearchPost handles search form submission and displays results.
// Supports filtering by category and displays enriched post data.
func (h *Handler) SearchPost(w http.ResponseWriter, r *http.Request) {
	query, categoryStr := strings.TrimSpace(r.FormValue("query")), r.FormValue("category")
	if query == "" {
		http.Redirect(w, r, "/search", http.StatusSeeOther)
		return
	}

	var categoryID *int
	if categoryStr != "" {
		if id, err := strconv.Atoi(categoryStr); err == nil {
			categoryID = &id
		}
	}

	posts, _ := h.DB.SearchPosts(query, categoryID)
	categories, _ := h.DB.GetCategories()
	user := h.getCurrentUser(r)

	// Add vote info
	if user != nil {
		for i := range posts {
			if vote, _ := h.DB.GetUserVoteForPost(user.ID, posts[i].ID); vote != nil {
				posts[i].UserLiked, posts[i].UserDisliked = *vote, !*vote
			}
		}
	}

	h.render(w, "base.html", struct {
		Page, SearchQuery, SelectedCategory string
		User                                *models.User
		Categories                          []models.Category
		Posts                               []models.Post
		ResultsCount                        int
	}{
		Page: "search", SearchQuery: query, SelectedCategory: categoryStr,
		User: user, Categories: categories, Posts: posts, ResultsCount: len(posts),
	})
}

// Members handles the members page displaying all forum users.
// Shows a list of all members with their statistics and profile links.
func (h *Handler) Members(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.MethodNotAllowed(w, r)
		return
	}

	user := h.getCurrentUser(r)

	// Get all users with their statistics
	users, err := h.DB.GetUsersWithStats()
	if err != nil {
		h.InternalServerError(w, r)
		return
	}

	// Get total counts for summary
	totalUsers := len(users)
	var totalPosts, totalComments int
	for _, u := range users {
		totalPosts += u.PostsCount
		totalComments += u.CommentsCount
	}

	h.render(w, "base.html", struct {
		Page          string
		User          *models.User
		Users         []models.UserWithStats
		TotalUsers    int
		TotalPosts    int
		TotalComments int
	}{
		Page:          "members",
		User:          user,
		Users:         users,
		TotalUsers:    totalUsers,
		TotalPosts:    totalPosts,
		TotalComments: totalComments,
	})
}

// Profile handles user profile page display with statistics and activity.
// Now supports both username-based URLs (/members/username) and ID-based URLs (/profile/123)
func (h *Handler) Profile(w http.ResponseWriter, r *http.Request) {
	var profileUser *models.User
	var err error

	// Check if this is a /members/username URL or /profile/id URL
	if strings.HasPrefix(r.URL.Path, "/members/") {
		// Extract username from /members/username
		username := strings.TrimPrefix(r.URL.Path, "/members/")
		if username == "" {
			h.NotFound(w, r)
			return
		}

		// Get user by username
		profileUser, err = h.DB.GetUserByUsername(username)
		if err != nil {
			h.NotFound(w, r)
			return
		}
	} else {
		// Original /profile/id logic for backward compatibility
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) != 3 {
			h.NotFound(w, r)
			return
		}

		userID, err := strconv.Atoi(pathParts[2])
		if err != nil {
			h.NotFound(w, r)
			return
		}

		profileUser, err = h.DB.GetUserByID(userID)
		if err != nil {
			h.NotFound(w, r)
			return
		}
	}

	stats, _ := h.DB.GetUserStats(profileUser.ID)
	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "posts"
	}

	var posts []models.Post
	var comments []models.CommentWithPost

	switch activeTab {
	case "posts":
		posts, _ = h.DB.GetUserPosts(profileUser.ID)
	case "liked":
		posts, _ = h.DB.GetUserLikedPosts(profileUser.ID)
	case "comments":
		comments, _ = h.DB.GetUserComments(profileUser.ID)
	}

	user := h.getCurrentUser(r)
	if user != nil && len(posts) > 0 {
		for i := range posts {
			if vote, _ := h.DB.GetUserVoteForPost(user.ID, posts[i].ID); vote != nil {
				posts[i].UserLiked, posts[i].UserDisliked = *vote, !*vote
			}
		}
	}

	h.render(w, "base.html", struct {
		Page, ActiveTab   string
		User, ProfileUser *models.User
		Stats             *models.UserStats
		Posts             []models.Post
		Comments          []models.CommentWithPost
	}{
		Page: "profile", ActiveTab: activeTab, User: user, ProfileUser: profileUser,
		Stats: stats, Posts: posts, Comments: comments,
	})
}

// NotFound handles 404 errors by rendering the error page.
func (h *Handler) NotFound(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	h.render(w, "error.html", struct{ User *models.User }{h.getCurrentUser(r)})
}

// InternalServerError handles 500 errors by rendering the error page.
func (h *Handler) InternalServerError(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusInternalServerError)
	h.render(w, "error.html", struct{ User *models.User }{h.getCurrentUser(r)})
}

// MethodNotAllowed handles 405 errors by rendering the error page.
func (h *Handler) MethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	h.render(w, "error.html", struct{ User *models.User }{h.getCurrentUser(r)})
}

// BadRequest handles 400 errors by rendering the error page.
func (h *Handler) BadRequest(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusBadRequest)
	h.render(w, "error.html", struct{ User *models.User }{h.getCurrentUser(r)})
}

// ImageModal handles image modal display for uploaded images.
// Validates image path and renders the modal template.
func (h *Handler) ImageModal(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.MethodNotAllowed(w, r)
		return
	}

	// Extract image path from URL
	imagePath := r.URL.Query().Get("image")
	if imagePath == "" {
		h.BadRequest(w, r)
		return
	}

	// Validate that the image exists and is in uploads directory
	if !strings.HasPrefix(imagePath, "uploads/") {
		h.BadRequest(w, r)
		return
	}

	// Check if file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		h.NotFound(w, r)
		return
	}

	// Render modal page
	h.render(w, "base.html", struct {
		Page      string
		ImagePath string
		User      *models.User
	}{
		Page:      "image-modal",
		ImagePath: imagePath,
		User:      h.getCurrentUser(r),
	})
}

// DeletePostConfirm handles the post deletion confirmation page.
// Validates user ownership and renders the confirmation template.
func (h *Handler) DeletePostConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.MethodNotAllowed(w, r)
		return
	}

	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postID := r.URL.Query().Get("id")
	if postID == "" {
		h.BadRequest(w, r)
		return
	}

	id, err := strconv.Atoi(postID)
	if err != nil {
		h.BadRequest(w, r)
		return
	}

	// Get post to verify ownership
	post, err := h.DB.GetPostByID(id)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if post.UserID != user.ID {
		h.BadRequest(w, r)
		return
	}

	h.render(w, "base.html", struct {
		Page  string
		Type  string
		Title string
		ID    int
		User  *models.User
	}{
		Page:  "delete-confirm",
		Type:  "post",
		Title: post.Title,
		ID:    post.ID,
		User:  user,
	})
}

// DeleteCommentConfirm handles the comment deletion confirmation page.
// Validates user ownership and renders the confirmation template.
func (h *Handler) DeleteCommentConfirm(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		h.MethodNotAllowed(w, r)
		return
	}

	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	commentID := r.URL.Query().Get("id")
	if commentID == "" {
		h.BadRequest(w, r)
		return
	}

	id, err := strconv.Atoi(commentID)
	if err != nil {
		h.BadRequest(w, r)
		return
	}

	// Get comment to verify ownership
	comment, err := h.DB.GetCommentByID(id)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if comment.UserID != user.ID {
		h.BadRequest(w, r)
		return
	}

	h.render(w, "base.html", struct {
		Page  string
		Type  string
		Title string
		ID    int
		User  *models.User
	}{
		Page:  "delete-confirm",
		Type:  "comment",
		Title: "Comment",
		ID:    comment.ID,
		User:  user,
	})
}

// DeleteComment handles actual comment deletion.
// Validates user ownership and removes the comment from the database.
func (h *Handler) DeleteComment(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.MethodNotAllowed(w, r)
		return
	}

	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	commentIDStr := r.FormValue("comment_id")
	if commentIDStr == "" {
		h.BadRequest(w, r)
		return
	}

	commentID, err := strconv.Atoi(commentIDStr)
	if err != nil {
		h.BadRequest(w, r)
		return
	}

	// Get comment to verify ownership
	comment, err := h.DB.GetCommentByID(commentID)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if comment.UserID != user.ID {
		h.BadRequest(w, r)
		return
	}

	// Delete the comment
	if err := h.DB.DeleteComment(commentID); err != nil {
		h.InternalServerError(w, r)
		return
	}

	// Redirect back to post with success message
	http.Redirect(w, r, fmt.Sprintf("/post/%d?deleted=true", comment.PostID), http.StatusSeeOther)
}

// DeletePost handles actual post deletion.
// Validates user ownership and removes the post and all associated data.
func (h *Handler) DeletePost(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		h.MethodNotAllowed(w, r)
		return
	}

	user := h.getCurrentUser(r)
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	postIDStr := r.FormValue("post_id")
	if postIDStr == "" {
		h.BadRequest(w, r)
		return
	}

	postID, err := strconv.Atoi(postIDStr)
	if err != nil {
		h.BadRequest(w, r)
		return
	}

	// Get post to verify ownership
	post, err := h.DB.GetPostByID(postID)
	if err != nil {
		h.NotFound(w, r)
		return
	}

	if post.UserID != user.ID {
		h.BadRequest(w, r)
		return
	}

	// Delete the post
	if err := h.DB.DeletePost(postID); err != nil {
		h.InternalServerError(w, r)
		return
	}

	// Redirect back to home with success message
	http.Redirect(w, r, "/?deleted=true", http.StatusSeeOther)
}
