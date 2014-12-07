package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"
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

	// Views
	views := []string{
		"topMovie",
		"risingMovie",
		"fallingMovie",
	}

	// Results
	results := make(map[string]template.JS)
	for _, view := range views {
		rows, _ := db.Query("SELECT * FROM " + view)
		defer rows.Close()
		results[view] = rowsToJson(rows)
	}

	// Generate index
	path := fmt.Sprintf("%s/index.html", dirOutput)
	w, err := os.Create(path)
	defer w.Close()
	data := make(map[string]interface{})
	for view, json := range results {
		data[view] = json
	}
	data["time"] = time.Now().UTC()
	path = fmt.Sprintf("%s/templates/index.html", dirTemplates)
	t, err := template.ParseFiles(path)
	if err != nil {
		fmt.Println(err)
	}
	t.Execute(w, &data)

	// Save json
	for view, json := range results {
		d1 := []byte(json)
		path := fmt.Sprintf("%s/%s.json", dirOutput, view)
		err := ioutil.WriteFile(path, d1, 0644)
		if err != nil {
			log.Println(err)
		} else {
			log.Println(path)
		}
	}
}
