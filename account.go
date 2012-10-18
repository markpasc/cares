package main

import (
	"github.com/jameskeane/bcrypt"
)

type Account struct {
	Id           int64  `db:"id"`
	Name         string `db:"name"`
	DisplayName  string `db:"displayName"`
	PasswordHash string `db:"passwordHash"`
}

var owner *Account

func NewAccount() *Account {
	return &Account{0, "", "", ""}
}

func AccountByName(name string) (*Account, error) {
	accounts, err := db.Select(Account{},
		"SELECT id, passwordHash, displayName FROM account WHERE name = $1 LIMIT 1",
		name)
	if err != nil {
		return nil, err
	}
	if len(accounts) > 0 {
		return accounts[0].(*Account), nil
	}
	return nil, nil
}

func LoadAccountForOwner() error {
	accounts, err := db.Select(Account{},
		"SELECT id, name, passwordHash, displayName FROM account ORDER BY id DESC LIMIT 1")
	if err != nil {
		return err
	}
	if len(accounts) > 0 {
		owner = accounts[0].(*Account)
	}
	return nil
}

func AccountForOwner() *Account {
	return owner
}

func (account *Account) HasPassword(pass string) bool {
	return bcrypt.Match(pass, account.PasswordHash)
}

func (account *Account) SetPassword(pass string) error {
	hash, err := bcrypt.Hash(pass)
	if err == nil {
		account.PasswordHash = hash
	}
	return err
}

func (account *Account) Save() error {
	if account.Id == 0 {
		return db.Insert(account)
	}
	_, err := db.Update(account)
	return err
}
