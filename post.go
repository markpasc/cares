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

type Author struct {
	Id   uint64 `col:"id"`
	Name string `col:"name"`
	Url  string `col:"url"`
}

func NewAuthor() (a *Author) {
	a = &Author{0, "", ""}
	return
}

func (a *Author) Save() error {
	return db.Save(a, "author")
}

func AuthorById(id uint64) (*Author, error) {
	row := db.QueryRow("SELECT name, url FROM author WHERE id = $1", id)

	var name string
	var url string
	err := row.Scan(&name, &url)
	if err != nil {
		logr.Errln("Error querying database for author #", id, ":", err.Error())
		return nil, err
	}

	author := &Author{id, name, url}
	return author, nil
}

type Post struct {
	Id       uint64         `col:"id"`
	AuthorId sql.NullInt64  `col:"author"`
	Url      sql.NullString `col:"url"`
	Html     string         `col:"html"`
	Posted   time.Time      `col:"posted"`
	Created  time.Time      `col:"created"`
	Deleted  pq.NullTime    `col:"deleted"`
}

func NewPost() (p *Post) {
	p = &Post{0, sql.NullInt64{0, false}, sql.NullString{"", false}, "", time.Now(), time.Now().UTC(), pq.NullTime{time.Unix(0, 0), false}}
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
	if !p.AuthorId.Valid {
		return nil, nil
	}
	return AuthorById(uint64(p.AuthorId.Int64))
}

func (p *Post) Slug() string {
	var binSlug [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(binSlug[0:binary.MaxVarintLen64], uint64(p.Id))
	slug := base32.StdEncoding.EncodeToString(binSlug[:n])
	return strings.TrimRight(slug, "=")
}

func (p *Post) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{
		"Id":        p.Id,
		"Html":      p.Html,
		"Permalink": p.Permalink(),
		"Created":   p.Created,
		"Posted":    p.Posted,
	}
	if p.AuthorId.Valid {
		author, err := p.Author()
		if err != nil {
			logr.Errln("Error loading author", p.AuthorId, "to marshal post", p.Id, ":", err.Error())
			// but continue
		} else {
			data["Author"] = map[string]interface{}{
				"Name": author.Name,
				"Url":  author.Url,
			}
		}
	}
	return json.Marshal(data)
}

func (p *Post) Save() error {
	return db.Save(p, "post")
}

func (p *Post) MarkDeleted() error {
	p.Deleted = pq.NullTime{time.Now().UTC(), true}
	return p.Save()
}

func PostById(id uint64) (*Post, error) {
	row := db.QueryRow("SELECT author, url, html, posted, created FROM post WHERE id = $1 AND deleted IS NULL", id)

	var author sql.NullInt64
	var url sql.NullString
	var html string
	var posted time.Time
	var created time.Time
	err := row.Scan(&author, &url, &html, &posted, &created)
	if err != nil {
		logr.Errln("Error querying database for post #", id, ":", err.Error())
		return nil, err
	}

	undeleted := pq.NullTime{time.Unix(0, 0), false}
	post := &Post{id, author, url, html, posted, created, undeleted}
	return post, nil
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

	id, n := binary.Uvarint(binSlug)
	if n <= 0 {
		return nil, fmt.Errorf("Read %d bytes decoding slug code %s", n, slug)
	}
	logr.Debugln("Yay, reckoned slug", slug, "is id", id, ", looking up")

	return PostById(id)
}

func FirstPost() (*Post, error) {
	logr.Debugln("Finding first post")
	row := db.QueryRow("SELECT id, author, url, html, posted, created FROM post WHERE deleted IS NULL ORDER BY posted ASC LIMIT 1")

	var id uint64
	var author sql.NullInt64
	var url sql.NullString
	var html string
	var posted time.Time
	var created time.Time
	err := row.Scan(&id, &author, &url, &html, &posted, &created)
	if err != nil {
		return nil, err
	}

	undeleted := pq.NullTime{time.Unix(0, 0), false}
	post := &Post{id, author, url, html, posted, created, undeleted}
	return post, nil
}

func postsForRows(rows *sql.Rows, count int) ([]*Post, error) {
	var err error

	posts := make([]*Post, 0, count)
	var id uint64
	var author sql.NullInt64
	var url sql.NullString
	var html string
	var posted time.Time
	var created time.Time
	undeleted := pq.NullTime{time.Unix(0, 0), false}

	i := 0
	for rows.Next() {
		err = rows.Scan(&id, &author, &url, &html, &posted, &created)
		if err != nil {
			logr.Errln("Error scanning row", i, ":", err.Error())
			return nil, err
		}

		if cap(posts) < i+1 {
			newPosts := make([]*Post, cap(posts), 2*cap(posts))
			copy(posts, newPosts)
			posts = newPosts
		}
		posts = posts[0 : i+1]
		posts[i] = &Post{id, author, url, html, posted, created, undeleted}
		i++
	}

	err = rows.Err()
	if err != nil {
		logr.Errln("Error looking at rows:", err.Error())
		return nil, err
	}

	return posts, nil
}

func RecentPosts(count int) ([]*Post, error) {
	rows, err := db.Query("SELECT id, author, url, html, posted, created FROM post WHERE deleted IS NULL ORDER BY posted DESC LIMIT $1", count)
	if err != nil {
		logr.Errln("Error querying database for", count, "posts:", err.Error())
		return nil, err
	}

	logr.Debugln("Deserializing all the returned posts")
	return postsForRows(rows, count)
}

func PostsBefore(before time.Time, count int) ([]*Post, error) {
	rows, err := db.Query("SELECT id, author, url, html, posted, created FROM post WHERE posted < $1 AND deleted IS NULL ORDER BY posted DESC LIMIT $2",
		before, count)
	if err != nil {
		return nil, err
	}

	return postsForRows(rows, count)
}

func PostsOnDay(day time.Time) ([]*Post, error) {
	year, month, mday := day.Date()
	minTime := time.Date(year, month, mday, 0, 0, 0, 0, time.UTC)
	year, month, mday = day.AddDate(0, 0, 1).Date()
	maxTime := time.Date(year, month, mday, 0, 0, 0, 0, time.UTC)

	rows, err := db.Query("SELECT id, author, url, html, posted, created FROM post WHERE $1 <= posted AND posted < $2 AND deleted IS NULL ORDER BY posted DESC",
		minTime, maxTime)
	if err != nil {
		return nil, err
	}

	return postsForRows(rows, 10)
}
