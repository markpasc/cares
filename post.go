package main

import (
	"bytes"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"github.com/bmizerany/pq"
	"strings"
	"time"
)

type Writestream struct {
	Id        int64
	AccountId int64
	PostId    int64
	Posted    time.Time
}

func NewWritestream() (w *Writestream) {
	w = &Writestream{0, 0, 0, time.Now()}
	return
}

func (w *Writestream) Save() error {
	if w.Id == 0 {
		return db.Insert(w)
	}
	_, err := db.Update(w)
	return err
}

type Author struct {
	Id   int64
	Name string
	Url  string
}

func NewAuthor() (a *Author) {
	a = &Author{0, "", ""}
	return
}

func (a *Author) Save() error {
	if a.Id == 0 {
		return db.Insert(a)
	}
	_, err := db.Update(a)
	return err
}

func AuthorById(id int64) (*Author, error) {
	author, err := db.Get(Author{}, id)
	if err != nil {
		return nil, err
	}
	return author.(*Author), nil
}

type Post struct {
	Id       int64
	AuthorId int64
	Url      sql.NullString
	Html     string
	Posted   time.Time
	Created  time.Time
	Deleted  pq.NullTime
}

func NewPost() (p *Post) {
	p = &Post{0, 0, sql.NullString{"", false}, "", time.Now(), time.Now().UTC(), pq.NullTime{time.Unix(0, 0), false}}
	return
}

func (p *Post) Permalink() string {
	if p.Url.Valid {
		return p.Url.String
	}
	return fmt.Sprintf("/post/%s", p.Slug())
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

func (p *Post) PostedYmd() string {
	return p.Posted.Format("2006-01-02")
}

func (p *Post) PostedRSS() string {
	return p.Posted.UTC().Format(time.RFC1123)
}

func (p *Post) PostedRFC3339() string {
	return p.Posted.UTC().Format(time.RFC3339)
}

func (p *Post) HtmlXML() string {
	var buf bytes.Buffer
	xml.Escape(&buf, []byte(p.Html))
	return buf.String()
}

func (p *Post) Author() (*Author, error) {
	return AuthorById(p.AuthorId)
}

func (p *Post) Slug() string {
	var binSlug [binary.MaxVarintLen64]byte
	n := binary.PutVarint(binSlug[0:binary.MaxVarintLen64], int64(p.Id))
	slug := base32.StdEncoding.EncodeToString(binSlug[:n])
	return strings.TrimRight(slug, "=")
}

func (p *Post) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"Id":            p.Id,
		"Html":          p.Html,
		"Permalink":     p.Permalink(),
		"Created":       p.Created,
		"Posted":        p.Posted,
		"AuthorIsOwner": p.AuthorIsOwner(),
	}

	author, err := p.Author()
	if err != nil {
		logr.Errln("Error loading author", p.AuthorId, "to marshal post", p.Id, ":", err.Error())
		// but continue
	} else {
		data["Author"] = map[string]interface{}{
			"Id":   author.Id,
			"Name": author.Name,
			"Url":  author.Url,
		}
	}

	return json.MarshalIndent(data, "", "    ")
}

func (p *Post) AuthorIsOwner() bool {
	// TODO: look at the real owner of the site (somehow).
	return p.AuthorId == 1
}

func (p *Post) Save() error {
	if p.Id == 0 {
		return db.Insert(p)
	}
	_, err := db.Update(p)
	return err
}

func (p *Post) MarkDeleted() error {
	p.Deleted = pq.NullTime{time.Now().UTC(), true}
	return p.Save()
}

func PostById(id int64) (*Post, error) {
	post, err := db.Get(Post{}, id)
	if err != nil {
		return nil, err
	}
	return post.(*Post), nil
}

func PostBySlug(slug string) (*Post, error) {
	logr.Debugln("Finding post id from slug", slug)

	// The decoder will want an even multiple of 8 bytes.
	padLen := 8 - (len(slug) % 8)
	slug += strings.Repeat("=", padLen)

	binSlug, err := base32.StdEncoding.DecodeString(slug)
	if err != nil {
		return nil, err
	}

	id, n := binary.Varint(binSlug)
	if n <= 0 {
		return nil, fmt.Errorf("Read %d bytes decoding slug code %s", n, slug)
	}
	logr.Debugln("Yay, reckoned slug", slug, "is id", id, ", looking up")

	return PostById(id)
}

func FirstPost() (*Post, error) {
	logr.Debugln("Finding first post")
	posts, err := db.Select(Post{},
		"SELECT id, authorId, url, html, posted, created FROM post WHERE deleted IS NULL ORDER BY posted ASC LIMIT 1")
	if err != nil {
		return nil, err
	}
	post := posts[0].(*Post)
	return post, nil
}

func postsForRows(rows []interface{}) []*Post {
	posts := make([]*Post, len(rows))
	for i, row := range rows {
		posts[i] = row.(*Post)
	}
	return posts
}

func RecentPosts(count int) ([]*Post, error) {
	rows, err := db.Select(Post{},
		"SELECT p.id, p.authorId, p.url, p.html, p.posted, p.created FROM post p, writestream w WHERE p.id = w.postId AND p.deleted IS NULL ORDER BY p.posted DESC LIMIT $1",
		count)
	if err != nil {
		logr.Errln("Error querying database for", count, "posts:", err.Error())
		return nil, err
	}
	return postsForRows(rows), nil
}

func PostsBefore(before time.Time, count int) ([]*Post, error) {
	rows, err := db.Select(Post{},
		"SELECT p.id, p.authorId, p.url, p.html, p.posted, p.created FROM post p WHERE posted < $1 AND deleted IS NULL ORDER BY posted DESC LIMIT $2",
		before, count)
	if err != nil {
		return nil, err
	}
	return postsForRows(rows), nil
}

func PostsOnDay(day time.Time) ([]*Post, error) {
	year, month, mday := day.Date()
	minTime := time.Date(year, month, mday, 0, 0, 0, 0, time.UTC)
	year, month, mday = day.AddDate(0, 0, 1).Date()
	maxTime := time.Date(year, month, mday, 0, 0, 0, 0, time.UTC)

	rows, err := db.Select(Post{},
		"SELECT id, authorId, url, html, posted, created FROM post WHERE $1 <= posted AND posted < $2 AND deleted IS NULL ORDER BY posted DESC",
		minTime, maxTime)
	if err != nil {
		return nil, err
	}
	return postsForRows(rows), nil
}
