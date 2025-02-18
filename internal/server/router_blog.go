package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rltran-codex/myblog-api/internal/database"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type BlogPost struct {
	ID        int       `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Category  string    `json:"category"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func handleBlogPostRoute(w http.ResponseWriter, r *http.Request) {
	method := r.Method
	switch method {
	case http.MethodPost:
		createPost(w, r)
	case http.MethodPut:
		updatePost(w, r)
	case http.MethodGet:
		if id, ok := mux.Vars(r)["id"]; ok {
			getPost(w, r, id)
		} else {
			sendError(w, http.StatusBadRequest, "no id found in request")
		}
	case http.MethodDelete:
		deletePost(w, r)
	default:
		w.WriteHeader(http.StatusBadRequest)
	}
}

// Create blog post
func createPost(w http.ResponseWriter, r *http.Request) {
	var data BlogPost

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&data)
	if err != nil {
		logError(r, "unable to parse json body from request: %v", err)
		sendError(w, http.StatusBadRequest, "unable to parse request body")
		return
	}
	defer r.Body.Close()

	post := database.BlogPost{
		Title:        data.Title,
		Content:      data.Content,
		CategoryName: data.Category,
		Tags:         []database.Tag{},
	}

	dbContext := database.DB.WithContext(r.Context())
	// fetch all the tags, create them if they dont exist
	if err = buildTags(dbContext, data, &post); err != nil {
		logError(r, "unable to create tag: %+v", err)
		sendError(w, http.StatusInternalServerError, "unable to create tag")
		return
	}

	result := dbContext.Create(&post)
	if result.Error != nil || result.RowsAffected == 0 {
		logError(r, "unable to create new post: %+v. %v", post, result.Error)
		sendError(w, http.StatusInternalServerError, "there was an error trying to save the post.")
		return
	}

	dbContext.Find(&post, post.ID)
	data.ID = int(post.ID)
	data.CreatedAt = post.CreatedAt
	data.UpdatedAt = post.UpdatedAt

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	err = json.NewEncoder(w).Encode(data)
	if err != nil {
		log.Println("unable to write response body", err)
		return
	}
	logInfo(r, "created new post: %+v", post)
}

func getAllPosts(w http.ResponseWriter, r *http.Request) {
	var result *gorm.DB
	var posts []database.BlogPost

	term := r.URL.Query().Get("term")
	if r.URL.RequestURI() == "/posts" {
		result = database.DB.WithContext(r.Context()).
			Preload("Tags").
			Find(&posts)

		if result.Error != nil {
			logError(r, "could not fetch any posts. %v", result.Error)
			sendError(w, http.StatusNotFound, "unable to fetch all posts")
			return
		}
	} else if len(term) > 0 {
		searchTerm := "%" + term + "%"
		logInfo(r, "searching database with term: %s", term)
		dbContext := database.DB.WithContext(r.Context())
		/*
			-- search categories, tags, blog_post.title, blog_post.content
			select * from blog_posts bp
			join blog_post_tags bpt on bpt.id = bp.id
			join tags t on bpt.id = t.id
			where bp.title like ?
			or bp.content like ?
			or t.name like ?
			or bp.category_name like ?
		*/
		result := dbContext.Preload("Tags").Preload("Category").
			Where("LOWER(title) LIKE LOWER(?) OR LOWER(content) LIKE LOWER(?)", searchTerm, searchTerm).
			Or("id IN (?)", dbContext.Model(&database.BlogPost{}).
				Joins("JOIN blog_post_tags ON blog_posts.id = blog_post_tags.blog_post_id").
				Joins("JOIN tags ON tags.id = blog_post_tags.tag_id").
				Where("LOWER(tags.name) LIKE LOWER(?)", searchTerm).
				Select("blog_posts.id")).
			Or("category_name IN (?)", dbContext.Model(&database.Category{}).
				Where("LOWER(name) LIKE LOWER(?)", searchTerm).
				Select("name")).
			Find(&posts)

		if result.Error != nil || result.RowsAffected == 0 {
			logError(r, "could not fetch any posts with term: %s. %v", term, result.Error)
			sendError(w, http.StatusNotFound, "unable to find any posts with search term")
			return
		}
	}

	if len(posts) > 0 {
		logInfo(r, "sending %d posts", len(posts))
		sendGetResponse(w, posts)
	} else {
		logError(r, "invalid request made: %s", r.URL.RequestURI())
		sendError(w, http.StatusBadRequest, fmt.Sprintf("invalid request made: %s", r.URL.RequestURI()))
	}
}

// Read blog post
func getPost(w http.ResponseWriter, r *http.Request, id string) {
	var result *gorm.DB
	var posts []database.BlogPost

	if len(id) == 0 {
		logError(r, "unable to fulfill request: %s", r.URL.RequestURI())
		sendError(w, http.StatusBadRequest, "unable to fulfill request")
	}

	result = database.DB.WithContext(r.Context()).
		Preload("Tags").
		First(&posts, id)
	if result.Error != nil {
		logError(r, "error occurred trying to find post with id: %s. %v", id, result.Error)
		sendError(w, http.StatusInternalServerError, fmt.Sprintf("could not find post with id: %s", id))
		return
	}

	logInfo(r, "GET request successful: %+v", posts)
	sendGetResponse(w, posts)
}

// Update blog post
func updatePost(w http.ResponseWriter, r *http.Request) {
	var id string
	var ok bool
	if id, ok = mux.Vars(r)["id"]; !ok {
		sendError(w, http.StatusBadRequest, "no id was found in request")
		return
	}

	var data BlogPost
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	err := decoder.Decode(&data)
	if err != nil {
		logError(r, "unable to parse json body from request: %v", err)
		sendError(w, http.StatusBadRequest, "unable to parse request body")
		return
	}

	var post database.BlogPost
	var count int64
	dbContext := database.DB.WithContext(r.Context())

	result := dbContext.Clauses(clause.Locking{Strength: "SHARE"}).
		Preload("Tags").
		Preload("Category").
		Find(&post, id).
		Count(&count)

	if result.Error != nil {
		logError(r, "unable to fetch record with id: %s. %v", id, result.Error)
		sendError(w, http.StatusInternalServerError, "unable to complete update request")
		return
	} else if count == 0 {
		logError(r, "no record found for post with id: %s", id)
		sendError(w, http.StatusNotFound, fmt.Sprintf("no post found for id: %s", id))
		return
	}

	// unpackage req body into database.blogpost struct
	if len(data.Title) > 0 {
		if len(data.Title) > 150 {
			logError(r, "request exceeded Titl limit")
			sendError(w, http.StatusBadRequest, "title exceeds 150 characters.")
			return
		}
		post.Title = data.Title
	}
	if len(data.Content) > 0 {
		if len(data.Content) > 2000 {
			logError(r, "request exceeded Content limit")
			sendError(w, http.StatusBadRequest, "content exceeds 2000 characters.")
			return
		}
		post.Content = data.Content
	}
	if len(data.Category) > 0 {
		// check if category exists
		var category database.Category
		result = dbContext.Where("name = ?", data.Category).First(&category)
		if result.Error != nil {
			logError(r, "request gave an unknown category: %s", data.Category)
			sendError(w, http.StatusBadRequest, fmt.Sprintf("given category does not exist: %s", data.Category))
			return
		}
		post.CategoryName = category.Name
		post.Category = category
	}
	// add tags
	if err = buildTags(dbContext, data, &post); err != nil {
		logError(r, "unable to attach new tags to existing post: %v", err)
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}

	result = dbContext.Preload("Tags").Model(&post).Updates(&post)
	if result.Error != nil {
		logError(r, "unable to successfully update row id: %s. %v", id, result.Error)
		sendError(w, http.StatusInternalServerError, "unable to complete update request")
		return
	}

	sendGetResponse(w, []database.BlogPost{post})
	logInfo(r, "successfully updated row [id: %s] %+v", id, post)
}

// Delete blog post
func deletePost(w http.ResponseWriter, r *http.Request) {
	id, ok := mux.Vars(r)["id"]
	if !ok {
		sendError(w, http.StatusBadRequest, "no id found in request")
		return
	}

	var post database.BlogPost
	if err := database.DB.Preload("Tags").First(&post, id).Error; err != nil {
		logError(r, "no post found with id: %s. %v", id, err)
		sendError(w, http.StatusNotFound, fmt.Sprintf("no post found with id: %s", id))
		return
	}

	if err := database.DB.Delete(&post, id).Error; err != nil {
		logError(r, "unable to remove post with id: %s, error: %v", id, err)
		sendError(w, http.StatusInternalServerError, "unable to remove post")
		return
	}

	w.WriteHeader(http.StatusNoContent)
	logInfo(r, "removed post: %+v", post)
}

func sendError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	w.Write([]byte(msg))
}

func sendGetResponse(w http.ResponseWriter, posts []database.BlogPost) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if len(posts) == 0 {
		return
	}

	if len(posts) > 1 {
		postsJson := []BlogPost{}
		for _, v := range posts {
			postsJson = append(postsJson, *packageResp(v))
		}
		json.NewEncoder(w).Encode(postsJson)
		return
	}

	json.NewEncoder(w).Encode(packageResp(posts[0]))
}

func packageResp(data database.BlogPost) *BlogPost {
	pResp := &BlogPost{
		ID:        int(data.ID),
		Title:     data.Title,
		Content:   data.Content,
		Category:  data.CategoryName,
		Tags:      []string{},
		CreatedAt: data.CreatedAt,
		UpdatedAt: data.UpdatedAt,
	}

	for _, v := range data.Tags {
		pResp.Tags = append(pResp.Tags, v.Name)
	}

	return pResp
}

func buildTags(dbContext *gorm.DB, data BlogPost, post *database.BlogPost) error {
	contains := func(slice []database.Tag, item database.Tag) bool {
		for _, v := range slice {
			if v == item {
				return true
			}
		}

		return false
	}

	for _, t := range data.Tags {
		var tag database.Tag
		var count int64
		result := dbContext.Where("name = ?", t).Find(&tag).Count(&count)
		if result.Error != nil || count == 0 {
			tag = database.Tag{Name: t}
			if err := dbContext.Create(&tag).Error; err != nil {
				return err
			}
		}

		if !contains(post.Tags, tag) {
			post.Tags = append(post.Tags, tag)
		}
	}
	return nil
}
