package main

import (
	"os"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"net"
)

var db *sql.DB
var stmts map[string]*sql.Stmt
var scrapeid int
var base string

func scrapePage(url string) bool {

	log.Printf("Scraping page %s", url)
	document, err := goquery.NewDocument(url)
	if err != nil {
		log.Println("Problem connecting to The Pirate Bay")
		os.Exit(1)
	}
	rows := document.Find("tr:not(:last-child)")

	// /torrent/11316236/Into_the_Storm_(2014)_1080p_BrRip_x264_-_YIFY
	rex := regexp.MustCompile(`^/torrent/(\d*)/`)

	rows.Each(func(i int, row *goquery.Selection) {

		idRaw, _ := row.Find(".detLink").Attr("href")
		id, _ := strconv.Atoi(rex.FindStringSubmatch(idRaw)[1])
		seeders, _ := strconv.Atoi(row.Find("td:nth-child(3)").Text())
		leechers, _ := strconv.Atoi(row.Find("td:nth-child(4)").Text())

		// Scrape torrent if it doesn't exist in the database
		var exists bool
		stmts["torrentExists"].QueryRow(id).Scan(&exists)
		if !exists {
			scrapeTorrent(id)
		}

		// Save status to database
		stmts["statusInsert"].Exec(id, seeders, leechers, scrapeid)
	})

	return true
}

func matchToInt(expression string, haystack string) (number int) {
	rex := regexp.MustCompile(expression)
	matches := rex.FindStringSubmatch(haystack)
	if matches != nil {
		number, _ = strconv.Atoi(matches[1])
	}
	return number
}

func scrapeTorrent(id int) {

	log.Printf("Scraping torrent %d", id)
	url := fmt.Sprintf("%s/torrent/%d/", base, id)
	document, _ := goquery.NewDocument(url)
	html, _ := document.Html()

	// <div id="title">Grimm S04E06 HDTV x264-LOL [eztv]</div>
	name := strings.TrimSpace(document.Find("#title").Text())
	// <dt>Uploaded:</dt> <dd>2014-11-29 04:16:05 GMT</dd>
	uploadedRaw := document.Find("dt:contains('Uploaded:') + dd").Text()
	uploaded := strings.TrimSpace(uploadedRaw)
	// <dt>Size:</dt> <dd>305.37 MiB (320207763 Bytes)</dd>
	sizeRaw := document.Find("dt:contains('Size:') + dd").Text()
	size := matchToInt(`\((\d+).Bytes\)`, sizeRaw)
	// http://www.imdb.com/title/tt1840309/
	imdb := matchToInt(`imdb.com/title/tt(\d+)/`, html)

	if imdb == 0 {
		stmts["torrentInsert"].Exec(id, name, uploaded, size, nil, scrapeid)
		return
	}

	// Scrape movie if it doesn't exist in database
	var exists bool
	stmts["movieExists"].QueryRow(imdb).Scan(&exists)
	if !exists {
		movie, err := scrapeMovie(imdb)
		if err != nil {
			log.Println(err)
			stmts["movieInsert"].Exec(id, nil, nil, nil, nil, nil, nil, nil, nil, nil, scrapeid)
		} else {
			stmts["movieInsert"].Exec(
				id,
				movie["title"],
				movie["released"],
				movie["imdb_rating"],
				movie["imdb_votes"],
				movie["tomato_meter"],
				movie["tomato_reviews"],
				movie["tomato_user_meter"],
				movie["tomato_user_reviews"],
				movie["trailer"],
				scrapeid)
		}
	}

	stmts["torrentInsert"].Exec(id, name, uploaded, size, imdb, scrapeid)
}

func scrapeMovie(id int) (map[string]interface{}, error) {

	log.Printf("Scraping movie %07d", id)
	url := fmt.Sprintf("http://api.jpatterson.me/beacon/movie/tt%07d", id)
	resp, err := http.Get(url)
	if err != nil {
		log.Println("Problem connecting to beacon API")
		return nil, fmt.Errorf("Problem connecting to beacon API")
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	var data map[string]map[string]interface{}
	err = json.Unmarshal(body, &data)
	movie := data["movie"]
	if err != nil || !movie["found"].(bool) {
		log.Printf("IMDB id %d not found on beacon API", id)
		return nil, fmt.Errorf("IMDB id %d not found on beacon API", id)
	}

	// Convert strings to ints ("5,912" to 5912)
	keys := []string{
		"imdb_rating",
		"imdb_votes",
		"tomato_meter",
		"tomato_reviews",
		"tomato_user_meter",
		"tomato_user_reviews",
	}
	rex := regexp.MustCompile(`[^0-9]`)
	for _, key := range keys {
		str := movie[key].(string)
		str = rex.ReplaceAllString(str, "")
		number, _ := strconv.Atoi(str)
		movie[key] = number
	}

	// Extract trailer url from annoying iframe wrapper
	// src="https://www.youtube.com/embed/oNHQw96SxJY"
	if movie["trailer"] != nil {
		rex = regexp.MustCompile(`src="([^"]+)"`)
		matches := rex.FindStringSubmatch(movie["trailer"].(string))
		if matches != nil {
			movie["trailer"] = matches[1]
		}
	}

	return movie, nil
}

func dialTimeout(network, addr string) (net.Conn, error) {
    return net.DialTimeout(network, addr, 2 * time.Second)
}

func ping(url string) bool {
	transport := http.Transport{
	    Dial: dialTimeout,
	}
	client := http.Client{
	    Transport: &transport,
	}
	resp, err := client.Get(url)
	if err != nil || resp.StatusCode != 200 {
		return false
	}
	return true
}

func scrape() {

	log.Println("Starting scrape...")

	db = getDb()
	defer db.Close()

	// Prepare query
	queries := map[string]string{
		"torrentExists": "SELECT EXISTS(SELECT 1 FROM torrent WHERE id = $1)",
		"torrentInsert": "INSERT INTO torrent VALUES ($1, $2, $3, $4, $5, $6)",
		"movieExists":   "SELECT EXISTS(SELECT 1 FROM movie WHERE id = $1)",
		"movieInsert":   "INSERT INTO movie VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)",
		"statusInsert":  "INSERT INTO status VALUES ($1, $2, $3, $4)",
		"movieUpdate": `
			UPDATE movie
			SET
				title = $2,
				released = $3,
				imdb_rating = $4,
				imdb_votes = $5,
				tomato_meter = $6,
				tomato_reviews = $7,
				tomato_user_meter = $8,
				tomato_user_reviews = $9,
				trailer = $10,
				scrape = $11
			WHERE id = $1`,
	}
	stmts = make(map[string]*sql.Stmt)
	var err error
	for key, query := range queries {
		stmts[key], err = db.Prepare(query)
		if err != nil {
			log.Panicf("Error preparing query: %s", query)
		}
	}

	// Get scrapeid
	db.QueryRow("INSERT INTO scrape DEFAULT VALUES RETURNING id").Scan(&scrapeid)

	// Choose TPB base url that is up
	urls := []string{
		"http://thepiratebay.se",
		"http://thepiratebay.ac",
		"http://thepiratebay.cr",
	}
	connection := false
	start := time.Now()
	for {
		for _, url := range urls {
			log.Printf("Pinging %s", url)
			if ping(url) {
				base = url
				connection = true
				break
			}
			log.Println("Ping failed")
		}
		if connection || time.Since(start).Minutes() > 10 {
			break
		}
	}
	if !connection {
		log.Println("Was not able to establish connection, exiting")
		os.Exit(1)
	}
	log.Printf("%s chosen as base", base)

	// Scrape the top torrents of the categories
	categories := []int{201, 207}
	for _, category := range categories {
		for i := 0; i < 10; i++ {
			url := fmt.Sprintf("%s/browse/%d/%d/9/", base, category, i)
			scrapePage(url)
		}
	}

	// Update movies that need updating
	rows, _ := db.Query("SELECT id FROM movieNeedsUpdate")
	defer rows.Close()
	for rows.Next() {
		var id int
		rows.Scan(&id)
		movie, err := scrapeMovie(id)
		if err != nil {
			continue
		}
		stmts["movieUpdate"].Exec(
			id,
			movie["title"],
			movie["released"],
			movie["imdb_rating"],
			movie["imdb_votes"],
			movie["tomato_meter"],
			movie["tomato_reviews"],
			movie["tomato_user_meter"],
			movie["tomato_user_reviews"],
			movie["trailer"],
			scrapeid)
	}
}
