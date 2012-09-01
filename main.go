package main

import (
	"database/sql"
	"log"
	"net/http"
)

func main() {
	var err error
	db, err = sql.Open("postgres", "host=localhost dbname=cares sslmode=disable")
	if err == nil {
		// Try a query to make sure it worked.
		_, err = db.Query("SELECT 1")
	}
	if err != nil {
		log.Println("Error connecting to db:", err.Error())
		return
	}

	http.HandleFunc("/static/", static)
	http.HandleFunc("/rss", rss)
	http.HandleFunc("/rssCloud", rssCloud)
	http.HandleFunc("/post", post)
	//http.HandleFunc("/archive/", archive)
	http.HandleFunc("/post/", permalink)
	http.HandleFunc("/", index)

	log.Println("Ohai web servin'")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
