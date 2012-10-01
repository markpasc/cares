package main

import (
	"bytes"
	"container/list"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Import struct {
	Id         uint64 `col:"id"`
	PostId     uint64 `col:"post"`
	Source     string `col:"source"`
	Identifier string `col:"identifier"`
}

func NewImport() *Import {
	return &Import{0, 0, "", ""}
}

func ImportBySourceIdentifier(source, identifier string) (*Import, error) {
	row := db.QueryRow("SELECT id, post, source, identifier FROM import WHERE source = $1 AND identifier = $2 LIMIT 1",
		source, identifier)

	var id, postid uint64
	err := row.Scan(&id, &postid, &source, &identifier)
	if err != nil {
		return nil, err
	}

	i := &Import{id, postid, source, identifier}
	return i, nil
}

func (im *Import) Save() error {
	return db.Save(im, "import")
}

type Mutation struct {
	Start int
	End   int
	Html  string
}

type MutationList []Mutation

func (ml MutationList) Len() int {
	return len(ml)
}

func (ml MutationList) Less(i, j int) bool {
	mutI := ml[i]
	mutJ := ml[j]
	if mutI.Start < mutJ.Start {
		return true
	}
	if mutI.Start == mutJ.Start && mutI.End < mutJ.End {
		return true
	}
	return false
}

func (ml MutationList) Swap(i, j int) {
	ml[i], ml[j] = ml[j], ml[i]
}

func indicesForEntity(ent map[string]interface{}) (int, int) {
	indices := ent["indices"].([]interface{})
	s, e := indices[0].(float64), indices[1].(float64)
	return int(s), int(e)
}

func makeTweetMutations(data map[string]interface{}) MutationList {
	text := data["text"].(string)
	ents := data["entities"].(map[string]interface{})

	mutations := list.New()
	for _, entIf := range ents["user_mentions"].([]interface{}) {
		ent := entIf.(map[string]interface{})
		screenName := html.EscapeString(ent["screen_name"].(string))
		html := fmt.Sprintf(`<a href="https://twitter.com/%s" title="%s">@%s</a>`,
			screenName, html.EscapeString(ent["name"].(string)), screenName)
		start, end := indicesForEntity(ent)
		mutations.PushBack(Mutation{start, end, html})
	}
	for _, entIf := range ents["hashtags"].([]interface{}) {
		ent := entIf.(map[string]interface{})
		tagText := ent["text"].(string)
		html := fmt.Sprintf(`<a href="https://twitter.com/search?q=%%23%s">#%s</a>`,
			tagText, tagText)
		start, end := indicesForEntity(ent)
		mutations.PushBack(Mutation{start, end, html})
	}
	for _, urlEnts := range []interface{}{ents["urls"], ents["media"]} {
		if urlEnts == nil {
			continue
		}
		for _, entIf := range urlEnts.([]interface{}) {
			ent := entIf.(map[string]interface{})
			url := ent["expanded_url"].(string)
			if url == "" {
				url = ent["url"].(string)
			}
			start, end := indicesForEntity(ent)
			linkText := text[start:end]
			html := fmt.Sprintf(`<a href="%s">%s</a>`, url, linkText)
			mutations.PushBack(Mutation{start, end, html})
		}
	}

	// We don't strictly need to regexp this of course but the strings package
	// won't find *all* instances of a substring, only the first or last.
	// Sadface that we can't just use lookahead assertion too.
	ampsRE, _ := regexp.Compile(`&`)
	amps := ampsRE.FindAllStringIndex(text, -1)
	for _, ampIndices := range amps {
		rest := text[ampIndices[1]:]
		matched, _ := regexp.MatchString("^(?:lt|gt|amp);", rest)
		if !matched {
			mutations.PushBack(Mutation{ampIndices[0], ampIndices[1], "&amp;"})
		}
	}

	nlRE, _ := regexp.Compile(`\n`)
	nls := nlRE.FindAllStringIndex(text, -1)
	for _, nlIndices := range nls {
		mutations.PushBack(Mutation{nlIndices[0], nlIndices[1], "<br>\n"})
	}

	mutList := make(MutationList, mutations.Len())
	for i, el := 0, mutations.Front(); el != nil; i, el = i+1, el.Next() {
		mutList[i] = el.Value.(Mutation)
	}

	return mutList
}

func mutateTweetText(data map[string]interface{}) string {
	text := data["text"].(string)
	mutations := makeTweetMutations(data)
	sort.Sort(mutations)

	var buf bytes.Buffer
	i := 0
	for _, mutation := range mutations {
		if i < mutation.Start {
			buf.WriteString(text[i:mutation.Start])
		}
		buf.WriteString(mutation.Html)
		i = mutation.End
	}
	// Include any trailing plain text.
	buf.WriteString(text[i:])

	return buf.String()
}

func ImportJson(path string) {
	logr.Debugln("Importing from Twitter export", path)
	jsons, err := ioutil.ReadDir(path)
	if err != nil {
		logr.Errln("Error finding Twitter export", path, "to import:", err.Error())
		return
	}

	count := 0
	for _, fileinfo := range jsons {
		if fileinfo.IsDir() {
			continue
		}
		if !strings.HasSuffix(fileinfo.Name(), "json") {
			continue
		}

		datafilepath := filepath.Join(path, fileinfo.Name())
		datafile, err := os.Open(datafilepath)
		if err != nil {
			logr.Errln("Error opening Twitter export file", datafilepath, ":", err.Error())
			return
		}

		var data map[string]interface{}
		dec := json.NewDecoder(datafile)
		err = dec.Decode(&data)
		if err != nil {
			logr.Errln("Error unmarshaling Twitter export file", datafilepath, ":", err.Error())
			return
		}

		if replyId, ok := data["in_reply_to_status_id_str"]; ok && replyId != nil && replyId.(string) != "" {
			continue
		}
		if retweeted, ok := data["retweeted"]; ok && retweeted != nil && retweeted.(bool) {
			continue
		}

		tweetId := data["id_str"].(string)
		im, err := ImportBySourceIdentifier("twitter", tweetId)
		if err == sql.ErrNoRows {
			im = NewImport()
			im.Source = "twitter"
			im.Identifier = tweetId
		} else if err != nil {
			logr.Errln("Error searching for existing imported post (twitter,", tweetId, "):", err.Error())
			return
		}

		var post *Post
		if im.PostId != 0 {
			post, err = PostById(im.PostId)
			if err != nil {
				logr.Errln("Error loading already-imported post", im.PostId, "for twitter post", im.Identifier, ":", err.Error())
				return
			}
		} else {
			post = NewPost()
		}

		tweetDate := data["created_at"].(string)
		post.Posted, err = time.Parse(time.RubyDate, tweetDate)
		if err != nil {
			logr.Errln("Error parsing publish time", tweetDate, "for twitter post", tweetId, ":", err.Error())
			return
		}

		post.Html = mutateTweetText(data)

		// TODO: store the source?
		// TODO: store the geoplace

		err = post.Save()
		if err != nil {
			logr.Errln("Error saving imported post:", err.Error())
			return
		}

		im.PostId = post.Id
		err = im.Save()
		if err != nil {
			logr.Errln("Error saving import notation for post", im.PostId, ":", err.Error())
			return
		}

		logr.Debugln("Imported post (twitter,", im.Identifier, ")")
		count++
	}

	logr.Debugln("Imported", count, "posts")
}

func ImportThinkup(path string) {
	logr.Debugln("Importing from Thinkup export", path)
	port, err := os.Open(path)
	if err != nil {
		logr.Errln("Error opening", path, "for import:", err.Error())
		return
	}

	r := csv.NewReader(port)
	// There may be missing header columns, so turn off field count checking.
	r.FieldsPerRecord = -1

	head, err := r.Read()
	if err != nil {
		logr.Errln("Error reading from import file", path, ":", err.Error())
		return
	}

	count := 0
	for {
		record, err := r.Read()
		if err != nil {
			break
		}

		data := make(map[string]string)
		for i, field := range head {
			data[field] = record[i]
		}

		// TODO: import replies, once there's something reasonable to import them as.
		if data["in_reply_to_post_id"] != "" {
			logr.Debugln("Skipping post (twitter,", data["post_id"], ") as it is a reply")
			continue
		}
		// TODO: import repeats, once there's something reasonable to import them as.
		if data["in_retweet_of_post_id"] != "" {
			logr.Debugln("Skipping post (twitter,", data["post_id"], ") as it is a repeat")
			continue
		}

		// okay now what
		im, err := ImportBySourceIdentifier("twitter", data["post_id"])
		if err == sql.ErrNoRows {
			im = NewImport()
			im.Source = "twitter"
			im.Identifier = data["post_id"]
		} else if err != nil {
			logr.Errln("Error searching for existing imported post (twitter,", data["post_id"], "):", err.Error())
			return
		}

		var post *Post
		if im.PostId != 0 {
			post, err = PostById(im.PostId)
			if err != nil {
				logr.Errln("Error loading already-imported post", im.PostId, "for twitter post", im.Identifier, ":", err.Error())
				return
			}
		} else {
			post = NewPost()
		}

		post.Posted, err = time.Parse("2006-01-02 15:04:05", data["pub_date"])
		if err != nil {
			logr.Errln("Error parsing publish time", data["pub_date"], "for twitter post", data["post_id"], ":", err.Error())
			return
		}

		// TODO: make the links link.
		html := template.HTMLEscapeString(data["post_text"])
		html = strings.Replace(html, "\n", "<br>\n", -1)
		post.Html = html

		// TODO: store the source?
		// TODO: store the geoplace

		err = post.Save()
		if err != nil {
			logr.Errln("Error saving imported post:", err.Error())
			return
		}

		im.PostId = post.Id
		err = im.Save()
		if err != nil {
			logr.Errln("Error saving import notation for post", im.PostId, ":", err.Error())
			return
		}

		logr.Debugln("Imported post (twitter,", im.Identifier, ")")
		count++
	}
	if err != nil {
		logr.Errln("Error reading import records:", err.Error())
		return
	}

	logr.Debugln("Finished importing", count, "posts!")
}
