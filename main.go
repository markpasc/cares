package main

import (
	"bufio"
	"flag"
	"log"
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

func main() {
	var dsn string
	var makeaccount, initdb, upgradedb bool
	var importthinkup, importjson, backup, importbackup string
	var port int
	flag.StringVar(&dsn, "database", "dbname=cares sslmode=disable", "database connection info")
	flag.BoolVar(&makeaccount, "make-account", false, "create a new account interactively")
	flag.BoolVar(&initdb, "init-db", false, "initialize the database")
	flag.BoolVar(&upgradedb, "upgrade-db", false, "upgrade the database schema")
	flag.StringVar(&importthinkup, "import-thinkup", "", "path to a Thinkup CSV export to import")
	flag.StringVar(&importjson, "import-json", "", "path to a directory of Twitter JSON to import")
	flag.StringVar(&backup, "backup", "", "path to which to save a backup of the current tweets")
	flag.StringVar(&importbackup, "import-backup", "", "path to a cares backup to import")
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
	} else if importjson != "" {
		ImportJson(importjson)
	} else if importthinkup != "" {
		ImportThinkup(importthinkup)
	} else if importbackup != "" {
		ImportBackup(importbackup)
	} else if backup != "" {
		ExportBackup(backup)
	} else {
		ServeWeb(port)
	}
}
