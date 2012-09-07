package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/hoisie/mustache"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func authedForHeader(authHeader string) (bool, error) {
	if !strings.HasPrefix(authHeader, "Basic ") {
		return false, nil
	}
	logr.Debugln("Yay, client gave a Basic auth header")

	userpass, err := base64.StdEncoding.DecodeString(authHeader[6:])
	if err != nil {
		logr.Debugln("Oops, error decoding the client's Basic auth header:", err.Error())
		// but report it as Unauthorized, not an error
		return false, nil
	}
	userpassParts := strings.SplitN(string(userpass), ":", 2)
	username, pass := userpassParts[0], userpassParts[1]

	account, err := AccountByName(username)
	if err == sql.ErrNoRows {
		logr.Debugln("No such account %s", username)
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
		logr.Errln("Error checking auth information:", err.Error())
		http.Error(w, "error loading auth information", http.StatusInternalServerError)
	} else if !authed {
		w.Header().Set("WWW-Authenticate", "Basic realm=\"cares\"")
		http.Error(w, "authorization required", http.StatusUnauthorized)
	}
	return
}

func WriteRssForPosts(w http.ResponseWriter, r *http.Request, posts []*Post, titleFormat string) (err error) {
	var host string
	var port string
	if strings.Contains(r.Host, ":") {
		host, port, err = net.SplitHostPort(r.Host)
		if err != nil {
			return
		}
	} else {
		host = r.Host
		// TODO: set port appropriately if we're on HTTPS
		port = "80"
	}

	owner := AccountForOwner()
	firstPost, err := FirstPost()
	if err != nil {
		return
	}

	// TODO: somehow determine if we're on HTTPS or no?
	baseurlUrl := url.URL{"http", "", nil, r.Host, "/", "", ""}
	baseurl := strings.TrimRight(baseurlUrl.String(), "/")

	data := map[string]interface{}{
		"posts": posts,
		"OwnerName": owner.DisplayName,
		"Title": fmt.Sprintf(titleFormat, owner.DisplayName),
		"baseurl": baseurl,
		"host": host,
		"port": port,
		"FirstPost": firstPost,
	}
	logr.Debugln("Rendering RSS with baseurl of", baseurl)
	xml := mustache.RenderFile("html/rss.xml", data)
	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(xml))
	return
}

func rss(w http.ResponseWriter, r *http.Request) {
	posts, err := RecentPosts(10)
	if err != nil {
		logr.Errln("Error loading posts for RSS feed:", err.Error())
		http.Error(w, "error finding recent posts", http.StatusInternalServerError)
		return
	}

	err = WriteRssForPosts(w, r, posts, "%s")
	if err != nil {
		logr.Errln("Error building RSS for recent posts:", err.Error())
		http.Error(w, "error generating rss for recent posts", http.StatusInternalServerError)
	}
}

func archive(w http.ResponseWriter, r *http.Request) {
	//  /archive/2012/09/06/rss.xml
	// 0 1       2    3  4  5
	pathParts := strings.SplitN(r.URL.Path, "/", 6)
	if len(pathParts) != 6 {
		http.NotFound(w, r)
		return
	}

	var year, month, day int
	var err error
	year, err = strconv.Atoi(pathParts[2])
	if err == nil {
		month, err = strconv.Atoi(pathParts[3])
	}
	if err == nil {
		day, err = strconv.Atoi(pathParts[4])
	}
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// TODO: support HTML archives too
	if pathParts[5] != "rss.xml" {
		http.NotFound(w, r)
		return
	}

	archiveDate := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	posts, err := PostsOnDay(archiveDate)
	if err != nil {
		logr.Errln("Error getting posts for day", archiveDate, "from database:", err.Error())
		http.Error(w, "error finding posts for day", http.StatusInternalServerError)
		return
	}

	err = WriteRssForPosts(w, r, posts, archiveDate.Format("%s for _2 Jan 2006"))
	if err != nil {
		logr.Errln("Error generating RSS for date", archiveDate, ":", err.Error())
		http.Error(w, "error generating rss for date", http.StatusInternalServerError)
	}
}

func index(w http.ResponseWriter, r *http.Request) {
	if len(r.URL.Path) > 1 {
		// Actually some other unhandled URL, so 404.
		http.NotFound(w, r)
		return
	}

	posts, err := RecentPosts(10)
	if err != nil {
		logr.Errln("Error loading recent posts for home page:", err.Error())
	}

	owner := AccountForOwner()
	data := map[string]interface{}{
		"posts": posts,
		"Title": owner.DisplayName,
		"OwnerName": owner.DisplayName,
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
		"Title": "a post • " + owner.DisplayName,
		"OwnerName": owner.DisplayName,
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
	logr.Debugln("Serving static file", path)
	http.ServeFile(w, r, path)
}
