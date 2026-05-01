#  Literary Lions Forum

##  Project Summary

**Literary Lions Forum** is a web‑based platform tailored for book lovers. It allows users to create discussion posts, share images such as book covers or cosy reading moments, and take part in conversations organised by topic.

##  Features at a Glance

###  **User Login & Profiles**
- Secure sign‑up and login system  
- Cookie‑based session handling  
- Encrypted password storage  
- Personal dashboards with activity stats  

###  **Engaging Discussions**
- Create posts with optional category tags  
- **Image Uploads** – share snapshots of books, quotes, or reading spaces  
- Comment threads for interactive chats  
- Like / dislike reactions on posts and comments  
- **Content Control** – users can delete their own posts and comments  

###  **Category‑Based Organisation**
- **Six Dedicated Categories**  
  -  *General Discussion* – anything book‑ish  
  -  *Book Reviews* – your thoughts on recent reads  
  -  *Author Spotlights* – deep dives into writers and their works  
  -  *Recommendations* – suggest titles to others  
  -  *Book Club* – announcements for club events  
  -  *Genres* – genre‑specific threads  

###  **Search & Filters**
- Filter by user, category, or favourites  
- Full‑text search across titles and body text  
- Quick links to *My Posts* and *Liked Posts*  

###  **Community Tools**
- **Members Directory** to browse every user  
- Clean profile URLs (`/members/username`)  
- Per‑user statistics and history  
- Community dashboard with overall metrics  

###  **Responsive, Accessible UI**
- responsive layouts  
- Literary‑themed styling  
- **Zero JavaScript** – fully server‑rendered HTML/CSS  
- Modal viewer for full‑screen images  
- Keyboard‑friendly navigation  

###  **Robust Image Handling**
- Accepts JPG, PNG, GIF, WebP  
- 10 MB upload limit  
- Server‑side validation and processing  
- Click‑to‑zoom full‑screen viewing  
- Posts with images are clearly flagged  
- Unique filenames prevent clashes  

###  **System Monitoring**
- JSON health endpoint at `/health`  
- Lightweight `/ping` route for liveness checks  
- Docker health‑check integration  


## ⚡ Getting Started

### Prerequisites
- Docker & Docker Compose  
- Go 1.21 or later (for local dev)  

### 🐳 Run with Docker (recommended)

```bash
git clone https://gitea.kood.tech/rahmanamanifard/literary-lions
cd literary‑lions
docker‑compose up --build (or "docker compose up --build")
```

Open `http://localhost:8080` in your browser and check health at `http://localhost:8080/health`.

### 🛠️ Local Development

```bash
go mod download          # fetch deps
mkdir -p uploads         # create upload dir
go run .                 # start server
```

---

##  Technical Overview

### Tech Stack
- **Backend:** Go (`net/http`)  
- **Database:** SQLite via `go‑sqlite3`  
- **Frontend:** Go templates HTML + CSS (no JS)  
- **Containerisation:** Multi‑stage Docker build  
- **Storage:** Local filesystem for uploads  
- **Monitoring:** Built‑in health endpoints  

---

##  Database Structure

| Table | Purpose |
|-------|---------|
| `users` | Account credentials & profile info |
| `sessions` | Active login sessions |
| `categories` | Discussion categories |
| `posts` | User posts (optionally with images) |
| `comments` | Threaded replies |
| `post_likes` | Post reactions |
| `comment_likes` | Comment reactions |

**Image columns in `posts`:**

- `image_filename` – unique stored name  
- `image_original_name` – original filename for display  

---

##  Security Highlights
- Passwords hashed with SHA‑256 & salt  
- UUID‑based session IDs with expiry  
- Strict file‑type & size checks on uploads  
- Parameterised SQL throughout  
- Template escaping prevents XSS  
- JS‑free frontend limits attack surface  

---
##  API Routes
 

### Authentication
- `GET/POST /login`  
- `GET/POST /register`  
- `GET /logout`  

### Content
- `GET /` – Home feed  
- `GET/POST /create-post`  
- `GET /post/{id}`  
- `POST /create-comment`  
- `POST /like-post`  
- `POST /like-comment`  

### Community
- `GET /members/`  
- `GET /members/{username}`  
- `GET/POST /search`  

### Deletion
- `GET /delete-post-confirm`  
- `GET /delete-comment-confirm`  
- `POST /delete-post`  
- `POST /delete-comment`  

### Media
- `GET /image-modal`  
- `GET /uploads/{filename}`  


## 🐳 Docker Details

### Healthcheck
```yaml
healthcheck:
  test: ["CMD", "wget", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/health"]
  interval: 60s
  timeout: 10s
  retries: 3
  start_period: 40s
```

### Environment Variables
| Var | Default | Description |
|-----|---------|-------------|
| `PORT` | 8080 | HTTP port |
| `DB_PATH` | forum.db | SQLite file location |

### Volume Mounts
- `forum_data` – persistent DB  
- `forum_uploads` – uploaded images  


## Developers:

- Rahman Amanifard
- Fatemeh Soufian
- Anna Storozhenko