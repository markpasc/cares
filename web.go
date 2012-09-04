package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hoisie/mustache"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
)

func authedForHeader(authHeader string) (bool, error) {
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false, nil
	}
	log.Println("Yay, client gave a Basic auth header")

	userpass, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		log.Println("Oops, error decoding the client's Basic auth header:", err.Error())
		// but report it as Unauthorized, not an error
		return false, nil
	}
	userpassParts := strings.SplitN(string(userpass), ":", 2)
	username, pass := userpassParts[0], userpassParts[1]

	account, err := AccountByName(username)
	if err == sql.ErrNoRows {
		log.Println("No such account %s", username)
		return false, nil
	} else if err != nil {
		return false, err
	}

	return account.HasPassword(pass), nil
}

func IsAuthed(w http.ResponseWriter, r *http.Request) (authed bool) {
	authHeader := r.Header.Get("Authorization")
	authed, err := authedForHeader(authHeader)
	if err != nil {
		log.Println("Error checking auth information:", err.Error())
		http.Error(w, "error loading auth information", http.StatusInternalServerError)
	} else if !authed {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"cares\"")
		http.Error(w, "authorization required", http.StatusUnauthorized)
	}
	return
}

func rss(w http.ResponseWriter, r *http.Request) {
	posts, err := RecentPosts(10)
	if err != nil {
		log.Println("OOPS ERROR", err.Error())
	} else {
		log.Println("OHAI", len(posts), "posts")
	}

	// TODO: what does this do when the Host header has no port?
	host, port, err := net.SplitHostPort(r.Host)
	owner := AccountForOwner()

	baseurl, err := url.Parse("/")
	baseurl.Host = r.Host // including port
	// TODO: somehow determine if we're on HTTPS or no?
	baseurl.Scheme = "http"

	data := map[string]interface{}{
		"posts": posts,
		"title": owner.DisplayName,
		"baseurl": strings.TrimRight(baseurl.String(), "/"),
		"host": host,
		"port": port,
	}
	log.Println("Rendering RSS with baseurl of", data["baseurl"])
	xml := mustache.RenderFile("html/rss.xml", data)
	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(xml))
}

func index(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 1 {
		// Actually some other unhandled URL, so 404.
		http.NotFound(w, r)
		return
	}

	posts, err := RecentPosts(10)
	if err != nil {
		log.Println("OOPS ERROR", err.Error())
	} else {
		log.Println("OHAI", len(posts), "posts")
	}

	owner := AccountForOwner()
	data := map[string]interface{}{
		"posts": posts,
		"title": owner.DisplayName,
	}
	html := mustache.RenderFileInLayout("html/index.html", "html/base.html", data)
	w.Write([]byte(html))
}

func permalink(w http.ResponseWriter, r *http.Request) {
	idstr := r.URL.Path[len("/post/"):]
	post, err := PostBySlug(idstr)
	if err != nil {
		http.Error(w, fmt.Sprintf("invalid post %s: %s", idstr, err.Error()), http.StatusBadRequest)
		return
	}

	if r.Method == "DELETE" {
		if !IsAuthed(w, r) {
			return
		}

		post.MarkDeleted()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("{\"ok\":true}"))
		return
	}

	owner := AccountForOwner()
	data := map[string]interface{}{
		"post": post,
		"title": "a post • " + owner.DisplayName,
	}
	html := mustache.RenderFileInLayout("html/permalink.html", "html/base.html", data)
	w.Write([]byte(html))
}

func post(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST is required", http.StatusMethodNotAllowed)
		return
	}
	if !IsAuthed(w, r) {
		return
	}

	post := NewPost()
	html := r.FormValue("html")
	if html == "" {
		http.Error(w, "html value is required", http.StatusBadRequest)
	}

	post.Html = html
	err := post.Save()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	go NotifyRssCloud(fmt.Sprintf("http://%s/rss", r.Host))

	ret, err := json.Marshal(post)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(ret)
}

func static(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[1:]
	log.Println("Serving static file", path)
	http.ServeFile(w, r, path)
}
