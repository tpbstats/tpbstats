package main

import (
	"os"
	"database/sql"
	_ "github.com/lib/pq"
)

func getDb() *sql.DB {
	db, err := sql.Open("postgres", os.Getenv("DATABASE"))
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	return db
}