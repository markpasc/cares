package main

import (
	"database/sql"
	"github.com/jameskeane/bcrypt"
)

type Account struct {
	Id           uint64
	Name         string
	DisplayName  string
	passwordHash string
}

var owner *Account

func NewAccount() *Account {
	return &Account{0, "", "", ""}
}

func AccountByName(name string) (*Account, error) {
	row := db.QueryRow("SELECT id, passwordHash, displayName FROM account WHERE name = $1 LIMIT 1",
		name)

	var id uint64
	var passwordHash string
	var displayName string
	err := row.Scan(&id, &passwordHash, &displayName)
	if err != nil {
		return nil, err
	}

	account := &Account{id, name, displayName, passwordHash}
	return account, nil
}

func LoadAccountForOwner() error {
	row := db.QueryRow("SELECT id, name, passwordHash, displayName FROM account ORDER BY id DESC LIMIT 1")

	var id uint64
	var name string
	var passwordHash string
	var displayName string
	err := row.Scan(&id, &name, &passwordHash, &displayName)
	if err == sql.ErrNoRows {
		// That's okay. Leave the owner nil.
		return nil
	}
	if err != nil {
		return err
	}

	owner = &Account{id, name, displayName, passwordHash}
	return nil
}

func AccountForOwner() *Account {
	return owner
}

func (account *Account) HasPassword(pass string) bool {
	return bcrypt.Match(pass, account.passwordHash)
}

func (account *Account) SetPassword(pass string) error {
	hash, err := bcrypt.Hash(pass)
	if err == nil {
		account.passwordHash = hash
	}
	return err
}

func (account *Account) Save() (err error) {
	if account.Id == 0 {
		row := db.QueryRow("INSERT INTO account (name, passwordHash, displayName) values ($1, $2, $3) RETURNING id",
			account.Name, account.passwordHash, account.DisplayName)
		var id uint64
		err = row.Scan(&id)
		if err != nil {
			return
		}
		account.Id = id
	} else {
		_, err = db.Exec("UPDATE account SET name = $2, passwordHash = $3, displayName = $4 WHERE id = $1",
			account.Id, account.Name, account.passwordHash, account.DisplayName)
	}
	return
}
