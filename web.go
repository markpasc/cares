package main

import (
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

func isAuthed(w http.ResponseWriter, r *http.Request) (authed bool) {
	authed = false
	authHeader := r.Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Basic ") {
		log.Println("Yay, client gave a Basic auth header")
		userpass, err := base64.StdEncoding.DecodeString(authHeader[6:])
		if err == nil {
			userpassParts := strings.SplitN(string(userpass), ":", 2)
			user, pass := userpassParts[0], userpassParts[1]
			if user == "markpasc" && pass == "password" {
				log.Println("Yay, client authorized!")
				authed = true
			} else {
				log.Println("Oops, client gave a bad username:password pair")
			}
		} else {
			log.Println("Oops, error decoding the client's Basic auth header:", err.Error())
		}
	}

	if !authed {
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

	data := make(map[string]interface{})
	data["posts"] = posts
	data["title"] = "markpasc"

	host, _, err := net.SplitHostPort(r.Host)
	data["host"] = host

	baseurl, err := url.Parse("/")
	baseurl.Host = r.Host  // including port
	// TODO: somehow determine if we're on HTTPS or no?
	baseurl.Scheme = "http"
	data["baseurl"] = strings.TrimRight(baseurl.String(), "/")
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

	data := make(map[string]interface{})
	data["posts"] = posts
	data["title"] = "markpasc"
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
		if !isAuthed(w, r) {
			return
		}

		post.MarkDeleted()

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		w.Write([]byte("{\"ok\":true}"))
		return
	}

	data := make(map[string]interface{})
	data["post"] = post
	data["title"] = "markpasc • a post"
	html := mustache.RenderFileInLayout("html/permalink.html", "html/base.html", data)
	w.Write([]byte(html))
}

func post(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		w.Header().Set("Allow", "POST")
		http.Error(w, "POST is required", http.StatusMethodNotAllowed)
		return
	}
	if !isAuthed(w, r) {
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
