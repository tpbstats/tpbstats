package main

import (
	"os"
	"fmt"
	"log"
	"time"
	"io/ioutil"
	"database/sql"
	"encoding/json"
	"html/template"
	"path/filepath"
)

func rowsToMap(rows *sql.Rows) []map[string]interface{} {
	columns, _ := rows.Columns()
	values := make([]interface{}, len(columns))
	pointers := make([]interface{}, len(columns))
	associatives := make([]map[string]interface{}, 0)
	for i, _ := range values {
		pointers[i] = &values[i]
	}
	for rows.Next() {
		rows.Scan(pointers...)
		associative := make(map[string]interface{})
		for i, _ := range values {
			if value, ok := values[i].([]uint8); ok {
				values[i] = string(value[:])
			}
			associative[columns[i]] = values[i]
		}
		associatives = append(associatives, associative)
	}
	return associatives
}

func rowsToJson(rows *sql.Rows) template.JS {
	json, _ := json.Marshal(rowsToMap(rows))
	return template.JS(string(json))
}

func generate() {

	log.Println("Starting generate...")

	dirTemplates, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	dirOutput, _ := filepath.Abs(os.Getenv("OUTPUTDIR"))
	db := getDb()
	defer db.Close()
	
	// Generate index
	
	path := fmt.Sprintf("%s/index.html", dirOutput)
	w, err := os.Create(path)
	defer w.Close()
	rows, _ := db.Query("SELECT * FROM topMovie")
	data := map[string]interface{} {
		"topMovie": rowsToJson(rows),
		"time": time.Now().UTC(),
	}
	path = fmt.Sprintf("%s/templates/index.html", dirTemplates)
	t, err := template.ParseFiles(path)
	if (err != nil) {
		fmt.Println(err);
	}
	t.Execute(w, &data)

	// Generate json
	views := []string {
		"movieScrape",
		"top",
		"topMovie",
		"topMovieScrape",
	}
	for _, view := range views {
		rows, _ := db.Query("SELECT * FROM " + view)
		defer rows.Close()
		json := rowsToJson(rows)
		d1 := []byte(json)
		path := fmt.Sprintf("%s/%s.json", dirOutput, view)
		err := ioutil.WriteFile(path, d1, 0644)
		if (err != nil) {
			log.Println(err)
		} else {
			log.Println(path)
		}
	}
}