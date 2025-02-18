package server

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rltran-codex/myblog-api/internal/config"
	"github.com/rltran-codex/myblog-api/internal/database"
)

func Start() {
	mux := mux.NewRouter()
	server := &http.Server{
		Addr:         config.Config.Address,
		Handler:      mux,
		ReadTimeout:  time.Duration(int64(config.Config.ReadTimeout) * int64(time.Second)),
		WriteTimeout: time.Duration(int64(config.Config.WriteTimeout) * int64(time.Second)),
	}

	// mux.HandleFunc("/posts", HandleBlogPostRoute)
	mux.HandleFunc("/posts", getAllPosts).Methods(http.MethodGet)
	mux.HandleFunc("/posts", createPost).Methods(http.MethodPost)
	mux.HandleFunc("/posts/{id:[0-9]+$}", handleBlogPostRoute).Methods(http.MethodGet, http.MethodDelete, http.MethodPut)
	database.ConnectDB()
	log.Printf("starting server and listening: %s", server.Addr)
	log.Fatal(server.ListenAndServe().Error())
}

func logError(r *http.Request, format string, vars ...interface{}) {
	errMsg := fmt.Sprintf(format, vars...)
	log.Printf("ERROR - [%s] - %s", r.RemoteAddr, errMsg)
}

func logInfo(r *http.Request, format string, vars ...interface{}) {
	msg := fmt.Sprintf(format, vars...)
	log.Printf("INFO - [%s] - %s", r.RemoteAddr, msg)
}
