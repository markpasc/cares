package main

import (
	"database/sql"
	"fmt"
	"github.com/bmizerany/pq"
	"github.com/coopernurse/gorp"
	"io/ioutil"
	"strings"
	"time"
)

const (
	SCHEMA_VERSION = 1
)

type Database struct {
	*gorp.DbMap
}

var db *Database

type Version struct {
	Version  int       `db:"version"`
	Upgraded time.Time `db:"upgraded"`
}

func DatabaseVersion() (int, error) {
	// Look what version of the database we're on (and try a query to make
	// sure it worked anyway).
	versions, err := db.Select(Version{},
		"SELECT version FROM schema ORDER BY version DESC LIMIT 1")
	if err == nil {
		version := versions[0].(*Version)
		return version.Version, nil
	}

	// Table exists but no rows is version 1.
	if err == sql.ErrNoRows {
		return 1, nil
	}

	// Table doesn't exist is version 0.
	if pqerr, ok := err.(*pq.PGError); ok && pqerr.Get('C') == "42P01" {
		return 0, nil
	}

	return 0, err
}

func OpenDatabase(dsn string, upgrading bool) (err error) {
	sqldb, err := sql.Open("postgres", dsn)
	if err != nil {
		return
	}

	dbmap := &gorp.DbMap{Db: sqldb, Dialect: gorp.PostgresDialect{}}
	dbmap.AddTableWithName(Account{}, "account").SetKeys(true, "Id")
	dbmap.AddTableWithName(Author{}, "author").SetKeys(true, "Id")
	dbmap.AddTableWithName(Post{}, "post").SetKeys(true, "Id")
	dbmap.AddTableWithName(Writestream{}, "writestream").SetKeys(true, "Id")
	dbmap.AddTableWithName(RssCloud{}, "rsscloud").SetKeys(true, "Id")
	dbmap.AddTableWithName(Import{}, "import").SetKeys(true, "Id")
	dbmap.AddTableWithName(Version{}, "schema")

	db = &Database{dbmap}

	version, err := DatabaseVersion()
	if !upgrading && version != SCHEMA_VERSION {
		return fmt.Errorf("Database reports schema is version %d. Use --upgrade-db to upgrade to %d.",
			version, SCHEMA_VERSION)
	}

	return
}

func RunSqlFile(filename string) (err error) {
	schemaBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		return
	}

	trans, err := db.Begin()
	if err != nil {
		return
	}

	statements := strings.Split(string(schemaBytes), ";\n")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		_, err = trans.Exec(statement)
		if err != nil {
			trans.Rollback()
			return
		}
	}

	err = trans.Commit()
	return
}

func InitializeDatabase() {
	err := RunSqlFile("schema/base.sql")
	if err == nil {
		err = db.Insert(&Version{SCHEMA_VERSION, time.Now().UTC()})
	}
	if err != nil {
		logr.Errln("Error initializing database:", err.Error())
		return
	}

	// Then make the owner record too.
	MakeAccount()
}

func UpgradeDatabase() {
	version, err := DatabaseVersion()
	if err != nil {
		logr.Errln("Error finding database schema version:", err.Error())
		return
	}

	if version == SCHEMA_VERSION {
		logr.Errln("Database is already upgraded to current schema version", SCHEMA_VERSION)
		return
	}
	if version > SCHEMA_VERSION {
		logr.Errln("Database is upgraded past current schema version", SCHEMA_VERSION, ". Use a newer version of the software with this database.")
		return
	}

	migrations, err := ioutil.ReadDir("schema")
	if err != nil {
		logr.Errln("Error finding migrations:", err.Error())
		return
	}

	for version < SCHEMA_VERSION {
		version++

		var filename string
		for _, fileinfo := range migrations {
			maybeFilename := fileinfo.Name()
			if strings.HasPrefix(maybeFilename, fmt.Sprintf("%0.2d-", version)) {
				filename = maybeFilename
				break
			}
		}
		if filename == "" {
			logr.Errln("No migration found for schema version", version)
			return
		}

		err = RunSqlFile("schema/" + filename)
		if err != nil {
			logr.Errln("Error performing migration", filename, ":", err.Error())
			return
		}

		err = db.Insert(&Version{version, time.Now().UTC()})
		if err != nil {
			logr.Errln("Error recording upgrade to schema version", version, ":", err.Error())
			return
		}
	}
}
