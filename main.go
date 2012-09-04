package main

import (
	"bufio"
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"
	"strings"
)

func Prompt(prompt string) (ret string) {
	input := bufio.NewReader(os.Stdin)
	os.Stdout.Write([]byte(prompt))
	for {
		line, isPrefix, err := input.ReadLine()
		if err != nil {
			return
		}
		ret += string(line)
		if !isPrefix {
			break
		}
	}
	ret = strings.TrimSpace(ret)
	return
}

func MakeAccount() {
	name := Prompt("Name: ")
	pass := Prompt("Password: ")
	displayName := Prompt("Display name: ")

	account := NewAccount()
	account.Name = name
	account.DisplayName = displayName
	account.SetPassword(pass)
	err := account.Save()
	if err != nil {
		log.Println("Error saving new account:", err.Error())
	}
}

func ServeWeb() {
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

func main() {
	var makeaccount bool
	flag.BoolVar(&makeaccount, "make-account", false, "create a new account interactively")
	flag.Parse()

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
	err = LoadAccountForOwner()
	if err != nil {
		log.Println("Error loading site owner:", err.Error())
		return
	}

	if makeaccount {
		MakeAccount()
	} else {
		ServeWeb()
	}
}
