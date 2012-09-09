package main

import (
	"bufio"
	"flag"
	"fmt"
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
		logr.Errln("Error saving new account:", err.Error())
	}
}

func ServeWeb(port int) {
	err := LoadAccountForOwner()
	if err != nil {
		logr.Errln("Error loading site owner:", err.Error())
		return
	}

	http.HandleFunc("/static/", static)
	http.HandleFunc("/rss", rss)
	http.HandleFunc("/rssCloud", rssCloud)
	http.HandleFunc("/post", post)
	http.HandleFunc("/activity", activity)
	http.HandleFunc("/archive/", archive)
	http.HandleFunc("/post/", permalink)
	http.HandleFunc("/", index)

	logr.Debugln("Ohai web servin'")
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

func main() {
	var dsn string
	var makeaccount, initdb, upgradedb bool
	var importthinkup string
	var port int
	flag.StringVar(&dsn, "database", "dbname=cares sslmode=disable", "database connection info")
	flag.BoolVar(&makeaccount, "make-account", false, "create a new account interactively")
	flag.BoolVar(&initdb, "init-db", false, "initialize the database")
	flag.BoolVar(&upgradedb, "upgrade-db", false, "upgrade the database schema")
	flag.StringVar(&importthinkup, "import-thinkup", "", "path to a Thinkup CSV export to import")
	flag.IntVar(&port, "port", 8080, "port on which to serve the web interface")
	flag.Parse()

	err := SetUpLogger()
	if err != nil {
		log.Println("Error setting up logging:", err.Error())
		return
	}
	defer logr.Close()

	err = OpenDatabase(dsn, upgradedb)
	if err != nil {
		logr.Errln("Error connecting to database:", err.Error())
		return
	}

	if initdb {
		InitializeDatabase()
	} else if upgradedb {
		UpgradeDatabase()
	} else if makeaccount {
		MakeAccount()
	} else if importthinkup != "" {
		ImportThinkup(importthinkup)
	} else {
		ServeWeb(port)
	}
}
