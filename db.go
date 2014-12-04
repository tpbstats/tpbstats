package main

import (
	"os"
	"fmt"
	"regexp"
	"database/sql"
	_ "github.com/lib/pq"
)

func getDb() *sql.DB {

	// https://groups.google.com/d/msg/golang-nuts/0XNrQRFuvoc/qDH_vXqN8j0J
	// postgres://user:password@host:port/dbname
	rex := regexp.MustCompile("(?i)^postgres://(?:([^:@]+):([^@]*)@)?([^@/:]+):(\\d+)/(.*)$")
	matches := rex.FindStringSubmatch(os.Getenv("DATABASE_URL"))
	spec := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s", matches[1], matches[2], matches[3], matches[4], matches[5])

	db, err := sql.Open("postgres", spec)
	if err != nil {
		panic(err)
	}
	err = db.Ping()
	if err != nil {
		panic(err)
	}
	return db
}