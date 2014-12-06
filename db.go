package main

import (
	"os"
	"log"
	"database/sql"
	_ "github.com/lib/pq"
)

func getDb() *sql.DB {
	db, err := sql.Open("postgres", os.Getenv("DATABASE"))
	if err != nil {
		log.Panic(err)
	}
	err = db.Ping()
	if err != nil {
		log.Panic(err)
	}
	return db
}