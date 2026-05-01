package main

import (
	"literary-lions/database"
	"literary-lions/handlers"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	// Initialize database
	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "forum.db"
	}
	db, err := database.NewDB(dbPath)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	// Initialize schema
	err = db.InitSchema()
	if err != nil {
		log.Fatal("Failed to initialize schema:", err)
	}

	// Initialize handlers
	h := handlers.NewHandler(db)

	// Create uploads directory for images
	err = os.MkdirAll("uploads", 0755)
	if err != nil {
		log.Fatal("Failed to create uploads directory:", err)
	}

	// Serve static files from the templates/static directory
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("templates/static"))))

	// Health check endpoints - ADD THESE FIRST
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"healthy","service":"literary-lions-forum","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	http.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	})

	// Setup existing routes
	http.HandleFunc("/", h.Home)
	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			h.RegisterGet(w, r)
		case "POST":
			h.RegisterPost(w, r)
		default:
			h.MethodNotAllowed(w, r)
		}
	})
	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			h.LoginGet(w, r)
		case "POST":
			h.LoginPost(w, r)
		default:
			h.MethodNotAllowed(w, r)
		}
	})
	http.HandleFunc("/logout", h.Logout)
	http.HandleFunc("/create-post", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			h.CreatePostGet(w, r)
		case "POST":
			h.CreatePostPost(w, r)
		default:
			h.MethodNotAllowed(w, r)
		}
	})
	http.HandleFunc("/post/", h.PostDetail)
	http.HandleFunc("/create-comment", h.CreateComment)
	http.HandleFunc("/like-post", h.LikePost)
	http.HandleFunc("/like-comment", h.LikeComment)

	// Search routes
	http.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			h.SearchGet(w, r)
		case "POST":
			h.SearchPost(w, r)
		default:
			h.MethodNotAllowed(w, r)
		}
	})

	// Members list page - exact match only
	http.HandleFunc("/members/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			h.MethodNotAllowed(w, r)
			return
		}

		// Check if this is exactly /members or /members/ (members list)
		if r.URL.Path == "/members" || r.URL.Path == "/members/" {
			h.Members(w, r) // Members list page
		} else {
			// This is /members/username - route to profile
			h.Profile(w, r) // Individual profile page
		}
	})

	// Profile routes - OLD: Keep for backward compatibility
	http.HandleFunc("/profile/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			h.Profile(w, r)
		} else {
			h.MethodNotAllowed(w, r)
		}
	})

	// Image modal route - replaces JavaScript modal
	http.HandleFunc("/image-modal", h.ImageModal)

	// Delete confirmation routes - replace JavaScript confirm dialogs
	http.HandleFunc("/delete-post-confirm", h.DeletePostConfirm)
	http.HandleFunc("/delete-comment-confirm", h.DeleteCommentConfirm)
	http.HandleFunc("/delete-post", h.DeletePost)
	http.HandleFunc("/delete-comment", h.DeleteComment)

	// Static file serving for uploaded images
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Handle 404s for all other routes
	http.HandleFunc("/favicon.ico", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	// Catch-all handler for 404s
	http.HandleFunc("/404", h.NotFound)

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting Literary Lions Forum on port %s", port)
	log.Printf("Visit http://localhost:%s to access the forum", port)

	err = http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
