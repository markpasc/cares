package main

import (
	"bytes"
	"database/sql"
	"encoding/base32"
	"encoding/binary"
	"encoding/xml"
	"fmt"
	"github.com/bmizerany/pq"
	"log"
	"strings"
	"time"
)

var db *sql.DB

type Post struct {
	Id      uint64
	Html    string
	Posted  time.Time
	Created time.Time
	Deleted pq.NullTime
}

func NewPost() (p *Post) {
	p = &Post{0, "", time.Now(), time.Now().UTC(), pq.NullTime{time.Unix(0, 0), false}}
	return
}

func (p *Post) Permalink() string {
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

func (p *Post) PostedRSS() string {
	return p.Posted.UTC().Format(time.RFC1123)
}

func (p *Post) HtmlXML() string {
	var buf bytes.Buffer
	xml.Escape(&buf, []byte(p.Html))
	return buf.String()
}

func (p *Post) Slug() string {
	var binSlug [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(binSlug[0:binary.MaxVarintLen64], uint64(p.Id))
	slug := base32.StdEncoding.EncodeToString(binSlug[:n])
	return strings.TrimRight(slug, "=")
}

func (p *Post) Save() (err error) {
	if p.Id == 0 {
		//var result sql.Result
		//result, err = db.Exec("INSERT INTO post (html, posted) VALUES ($1, $2) RETURNING id",
		row := db.QueryRow("INSERT INTO post (html, posted, created, deleted) VALUES ($1, $2, $3, $4) RETURNING id",
			p.Html, p.Posted, p.Created, p.Deleted)
		var id uint64
		err = row.Scan(&id)
		if err != nil {
			return err
		}
		p.Id = id
	} else {
		_, err = db.Exec("UPDATE post SET html = $2, posted = $3, created = $4, deleted = $5 WHERE id = $1",
			p.Id, p.Html, p.Posted, p.Created, p.Deleted)
	}
	return nil
}

func (p *Post) MarkDeleted() (err error) {
	p.Deleted = pq.NullTime{time.Now().UTC(), true}
	return p.Save()
}

func PostById(id uint64) (*Post, error) {
	row := db.QueryRow("SELECT html, posted, created FROM post WHERE id = $1 AND deleted IS NULL", id)

	var html string
	var posted time.Time
	var created time.Time
	err := row.Scan(&html, &posted, &created)
	if err != nil {
		log.Println("Error querying database for post #", id, ":", err.Error())
		return nil, err
	}

	deleted := pq.NullTime{time.Unix(0, 0), false}
	post := &Post{id, html, posted, created, deleted}
	return post, nil
}

func PostBySlug(slug string) (*Post, error) {
	log.Println("Finding post id from slug", slug)

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
	log.Println("Yay, reckoned slug", slug, "is id", id, ", looking up")

	return PostById(id)
}

func RecentPosts(count int) ([]*Post, error) {
	rows, err := db.Query("SELECT id, html, posted, created FROM post WHERE deleted IS NULL ORDER BY posted DESC LIMIT 10")
	if err != nil {
		log.Println("Error querying database for", count, "posts:", err.Error())
		return nil, err
	}

	log.Println("Deserializing all the returned posts")
	posts := make([]*Post, 0, count)
	var id uint64
	var html string
	var posted time.Time
	var created time.Time
	i := 0
	for rows.Next() {
		err = rows.Scan(&id, &html, &posted, &created)
		if err != nil {
			log.Println("Error scanning row", i, ":", err.Error())
			return nil, err
		}

		posts = posts[0 : i+1]
		posts[i] = &Post{id, html, posted, created, pq.NullTime{time.Unix(0, 0), false}}
		i++
	}

	err = rows.Err()
	if err != nil {
		log.Println("Error lookin' at rows:", err.Error())
		return nil, err
	}

	return posts, nil
}
