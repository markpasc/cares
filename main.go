package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"time"
	"database/sql"
	"strconv"
	_ "github.com/jbarham/gopgsqldriver"
	"github.com/hoisie/mustache"
)

var db *sql.DB

type Post struct {
	Id int
	Html string
	Posted time.Time
}

func NewPost() (p *Post) {
	p = &Post{0, "", time.Now()}
	return
}

func (p *Post) Permalink() string {
	return fmt.Sprintf("/%d/%d", p.Posted.Year(), p.Id)
}

func (p *Post) PostedTime() string {
	return p.Posted.Format("3:04")
}

func (p *Post) PostedAM() string {
	return p.Posted.Format("PM")
}

func (p *Post) PostedDate() string {
	return p.Posted.Format("_2 Jan 2006")
}

func (p *Post) PostedRSS() string {
	return p.Posted.UTC().Format(time.RFC1123)
}

func (p *Post) HtmlXML() string {
	var buf bytes.Buffer
	xml.Escape(&buf, []byte(p.Html))
	return buf.String()
}

func (p *Post) Save() (err error) {
	if p.Id == 0 {
		//var result sql.Result
		//result, err = db.Exec("INSERT INTO post (html, posted) VALUES ($1, $2) RETURNING id",
		row := db.QueryRow("INSERT INTO post (html, posted) VALUES ($1, $2) RETURNING id",
			p.Html, p.Posted)
		var id int
		err = row.Scan(&id)
		if (err != nil) {
			return err
		}
		p.Id = id
	} else {
		_, err = db.Exec("UPDATE post SET html = $1, posted = $2 WHERE id = $3",
			p.Html, p.Posted, p.Id)
	}
	return nil
}

func PostById(id int) (*Post, error) {
	row := db.QueryRow("SELECT html, posted FROM post WHERE id = $1", id)

	var html string
	var posted string
	err := row.Scan(&html, &posted)
	if (err != nil) {
		log.Println("Error querying database for post #", id, ":", err.Error())
		return nil, err
	}
	log.Println("Got time string:", posted)
	postedTime, err := time.Parse("2006-01-02 15:04:05.000000", posted)
	if (err != nil) {
		log.Println("Error converting database date", posted, "to a time:", err.Error())
		return nil, err
	}

	post := &Post{id, html, postedTime}
	return post, nil
}

func RecentPosts(count int) ([]*Post, error) {
	rows, err := db.Query("SELECT * FROM post ORDER BY posted DESC LIMIT 10")
	if (err != nil) {
		log.Println("Error querying database for", count, "posts:", err.Error())
		return nil, err
	}

	log.Println("Deserializing all the returned posts")
	posts := make([]*Post, 0, count)
	var id int
	var html string
	var posted string
	var postedTime time.Time
	i := 0
	for rows.Next() {
		err = rows.Scan(&id, &html, &posted)
		if (err != nil) {
			log.Println("Error scanning row", i, ":", err.Error())
			return nil, err
		}

		log.Println("Got time string", posted)
		postedTime, err = time.Parse("2006-01-02 15:04:05.000000", posted)
		posts = posts[0:i+1]
		posts[i] = &Post{id, html, postedTime}
		i++
	}

	err = rows.Err()
	if (err != nil) {
		log.Println("Error lookin' at rows:", err.Error())
		return nil, err
	}

	return posts, nil
}

func rss(w http.ResponseWriter, r *http.Request) {
	posts, err := RecentPosts(10)
	if (err != nil) {
		log.Println("OOPS ERROR", err.Error())
	} else {
		log.Println("OHAI", len(posts), "posts")
	}

	data := make(map[string] interface{})
	data["posts"] = posts
	data["title"] = "markpasc"

	baseurl, err := url.Parse("/")
	baseurl.Host = r.Host
	// TODO: somehow determine if we're on HTTPS or no?
	baseurl.Scheme = "http"
	baseurl.Fragment = ""
	data["baseurl"] = strings.TrimRight(baseurl.String(), "/")
	log.Println("Rendering RSS with baseurl of", data["baseurl"])

	xml := mustache.RenderFile("html/rss.xml", data)
	w.Header().Set("Content-Type", "application/rss+xml")
	w.Write([]byte(xml))
}

func index(w http.ResponseWriter, r *http.Request) {
	posts, err := RecentPosts(10)
	if (err != nil) {
		log.Println("OOPS ERROR", err.Error())
	} else {
		log.Println("OHAI", len(posts), "posts")
	}

	data := make(map[string] interface{})
	data["posts"] = posts
	data["title"] = "markroblog"
	html := mustache.RenderFileInLayout("html/index.html", "html/base.html", data)
	w.Write([]byte(html))
}

func permalink(w http.ResponseWriter, r *http.Request) {
	idstr := r.URL.Path[len("/post/"):]
	log.Println("Finding id from path component", idstr)
	id, err := strconv.ParseInt(idstr, 10, 32)
	if (err != nil) {
		http.Error(w, "invalid post id: " + err.Error(), http.StatusBadRequest)
		return
	}

	var post *Post
	post, err = PostById(int(id))
	if (err != nil) {
		http.Error(w, fmt.Sprintf("invalid post #%d: %s", id, err.Error()), http.StatusBadRequest)
		return
	}

	data := make(map[string] interface{})
	data["post"] = post
	data["title"] = "markroblog • a post"
	html := mustache.RenderFileInLayout("html/permalink.html", "html/base.html", data)
	w.Write([]byte(html))
}

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

func makepost(w http.ResponseWriter, r *http.Request) {
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

func main() {
	var err error
	db, err = sql.Open("postgres", "host=localhost dbname=cares")
	if err == nil {
		// Try a query to make sure it worked.
		_, err = db.Query("SELECT 1")
	}
	if err != nil {
		log.Println("Error connecting to db:", err.Error())
		return
	}

	http.HandleFunc("/static/", static)
	http.HandleFunc("/2012/", permalink)
	http.HandleFunc("/post", makepost)
	http.HandleFunc("/rss", rss)
	http.HandleFunc("/", index)

	log.Println("Ohai web servin'")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
