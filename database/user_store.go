package database

import (
	"database/sql"
	_ "github.com/lib/pq"
)

type UserStore struct {
	DB *sql.DB
}

func NewUserStore(conn *sql.DB) *UserStore {
	usr := &UserStore{
		DB: conn,
	}
	return usr
}
