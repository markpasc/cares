package main

import (
	"database/sql"
	"fmt"
	"github.com/bmizerany/pq"
	"io/ioutil"
	"reflect"
	"strings"
)

const (
	SCHEMA_VERSION = 3
)

type Record interface {
	Save() error
}

type Database struct {
	*sql.DB
}

func (d *Database) Save(r Record, table string) (err error) {
	recValue := reflect.Indirect(reflect.ValueOf(r))
	recType := recValue.Type()

	// TODO: really should assert that recValue is a struct before we NumField it

	colNames := make([]string, 0, recValue.NumField()-1) // don't count ID
	colValues := make([]interface{}, 0, recValue.NumField())

	taggedColumns := 0
	var idValue *reflect.Value
	for i := 0; i < recValue.NumField(); i++ {
		value := recValue.Field(i)
		colTag := recType.Field(i).Tag.Get("col")
		if colTag == "id" {
			idValue = &value
		} else if colTag != "" {
			colNames = colNames[0 : taggedColumns+1]
			colNames[taggedColumns] = colTag
			colValues = colValues[0 : taggedColumns+1]
			colValues[taggedColumns] = value.Interface()
			taggedColumns++
		}
	}

	if idValue == nil {
		return fmt.Errorf("no id field")
	}

	colNameSet := strings.Join(colNames, ", ")
	placeholders := make([]string, len(colNames))
	for i, _ := range colNames {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	placeholderSet := strings.Join(placeholders, ", ")

	if idValue.Uint() == 0 {
		sqlText := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s) RETURNING id",
			table, colNameSet, placeholderSet)
		//logr.Debugln("SQL:", sqlText, "←", colValues)
		row := db.QueryRow(sqlText, colValues...)

		var id uint64
		err = row.Scan(&id)
		if err == nil {
			idValue.SetUint(id)
		}
	} else {
		sqlText := fmt.Sprintf("UPDATE %s SET (%s) = (%s) WHERE id = $%d",
			table, colNameSet, placeholderSet, len(colNames)+1)
		lasty := len(colValues)
		colValues = colValues[0 : lasty+1]
		colValues[lasty] = idValue.Uint()
		//logr.Debugln("SQL:", sqlText, "←", colValues)
		// Use Exec instead of Query here, so _ isn't a Rows holding a
		// database connection private until it's closed or fully read,
		// which of course it won't be since we don't save the Rows. Oops.
		_, err = db.Exec(sqlText, colValues...)
	}
	return
}

var db *Database

func DatabaseVersion() (version int, err error) {
	// Look what version of the database we're on (and try a query to make
	// sure it worked anyway).
	row := db.QueryRow("SELECT version FROM schema ORDER BY version DESC LIMIT 1")
	err = row.Scan(&version)
	if err != nil {
		// Table exists but no rows is version 1.
		if err == sql.ErrNoRows {
			return 1, nil
		}

		// Table doesn't exist is version 0.
		if pqerr, ok := err.(*pq.PGError); ok && pqerr.Get('C') == "42P01" {
			return 0, nil
		}
	}
	// Whether version or err were set, return those.
	return
}

func OpenDatabase(dsn string, upgrading bool) (err error) {
	sqldb, err := sql.Open("postgres", dsn)
	if err != nil {
		return
	}

	db = &Database{sqldb}

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

	_, err = db.Query("BEGIN")
	if err != nil {
		return
	}

	statements := strings.Split(string(schemaBytes), ";\n")
	for _, statement := range statements {
		statement = strings.TrimSpace(statement)
		if statement == "" {
			continue
		}

		_, err = db.Query(statement)
		if err != nil {
			db.Query("ROLLBACK")
			return
		}
	}

	_, err = db.Query("COMMIT")
	return
}

func InitializeDatabase() {
	err := RunSqlFile("schema/base.sql")
	if err == nil {
		_, err = db.Query("INSERT INTO schema (version) VALUES ($1)", SCHEMA_VERSION)
	}
	if err != nil {
		logr.Errln("Error initializing database:", err.Error())
		return
	}

	_, err = db.Query("INSERT INTO schema (version) VALUES ($1)", SCHEMA_VERSION)
	if err != nil {
		logr.Errln("Error recording installed schema version", SCHEMA_VERSION, ":", err.Error())
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

		_, err = db.Query("INSERT INTO schema (version) VALUES ($1)", version)
		if err != nil {
			logr.Errln("Error recording upgrade to schema version", version, ":", err.Error())
			return
		}
	}
}
