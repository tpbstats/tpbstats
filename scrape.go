package main

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"encoding/json"
	"database/sql"
	"github.com/PuerkitoBio/goquery"
)

var db *sql.DB
var stmts map[string]*sql.Stmt
var scrapeid int

func scrapePage(url string) bool {

	document, err := goquery.NewDocument(url)
	if err != nil {
		log.Println("Problem connecting to The Pirate Bay")
		return false
	}
	rows := document.Find("tr:not(:last-child)")

	// /torrent/11316236/Into_the_Storm_(2014)_1080p_BrRip_x264_-_YIFY
	rexId := regexp.MustCompile("^/torrent/(\\d*)/")

	rows.Each(func(i int, row *goquery.Selection) {

		idRaw, _ := row.Find(".detLink").Attr("href")
		id, _ := strconv.Atoi(rexId.FindStringSubmatch(idRaw)[1])
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
	});

	return true
}

func matchToInt(expression string, haystack string) (number int) {
	rex := regexp.MustCompile(expression)
	matches := rex.FindStringSubmatch(haystack)
	if (matches != nil) {
		number, _ = strconv.Atoi(matches[1])
	}
	return number
}

func scrapeTorrent(id int) {

	url := fmt.Sprintf("http://thepiratebay.se/torrent/%d/", id)
	document, _ := goquery.NewDocument(url)
	html, _ := document.Html()

	// <div id="title">Grimm S04E06 HDTV x264-LOL [eztv]</div>
	name := strings.TrimSpace(document.Find("#title").Text())
	// <dt>Uploaded:</dt> <dd>2014-11-29 04:16:05 GMT</dd>
	uploadedRaw := document.Find("dt:contains('Uploaded:') + dd").Text()
	uploaded := strings.TrimSpace(uploadedRaw)
	// <dt>Size:</dt> <dd>305.37 MiB (320207763 Bytes)</dd>
	sizeRaw := document.Find("dt:contains('Size:') + dd").Text()
	size := matchToInt("\\((\\d+).Bytes\\)", sizeRaw)
	// http://www.imdb.com/title/tt1840309/
	imdb := matchToInt("imdb.com/title/tt(\\d+)/", html)

	if imdb == 0 {
		stmts["torrentInsert"].Exec(id, name, uploaded, size, nil, scrapeid)
		return
	}

	// Scrape movie if it doesn't exist in database
	var exists bool
	stmts["movieExists"].QueryRow(imdb).Scan(&exists)
	if !exists {
		if !scrapeMovie(imdb) {
			stmts["movieInsert"].Exec(id, nil, nil, nil, nil, nil, nil, nil, nil, scrapeid)
		}
	}

	stmts["torrentInsert"].Exec(id, name, uploaded, size, imdb, scrapeid)
}

func interToInt(inter interface{}) int {
	str := inter.(string);
	rex := regexp.MustCompile("[^0-9]")
	str = rex.ReplaceAllString(str, "")
	number, _ := strconv.Atoi(str)
	return number;
}

func scrapeMovie(id int) bool {

	url := fmt.Sprintf("http://api.jpatterson.me/beacon/movie/tt%07d", id)
	document, err := goquery.NewDocument(url)
	if err != nil {
		log.Println("Problem connecting to beacon API")
		return false
	}

	bytes := []byte(document.Text())
	var data map[string]map[string]interface{}
	err = json.Unmarshal(bytes, &data)
	movie := data["movie"]

	if err != nil || !movie["found"].(bool) {
		log.Printf("IMDB id %d not found on beacon API", id)
		return false
	}
	
	stmts["movieInsert"].Exec(
		id,
		movie["title"],
		movie["released"],
		interToInt(movie["imdb_rating"]),
		interToInt(movie["imdb_votes"]),
		interToInt(movie["tomato_meter"]),
		interToInt(movie["tomato_reviews"]),
		interToInt(movie["tomato_user_meter"]),
		interToInt(movie["tomato_user_reviews"]),
		scrapeid)

	return true
}

func scrape() {

	log.Println("Starting scrape...")

	db = getDb()
	defer db.Close()

	stmts = make(map[string]*sql.Stmt)
	stmts["torrentExists"], _ = db.Prepare("SELECT EXISTS(SELECT 1 FROM torrent WHERE id = $1)")
	stmts["torrentInsert"], _ = db.Prepare("INSERT INTO torrent VALUES ($1, $2, $3, $4, $5, $6)")
	stmts["movieExists"], _ = db.Prepare("SELECT EXISTS(SELECT 1 FROM movie WHERE id = $1)")
	stmts["movieInsert"], _ = db.Prepare("INSERT INTO movie VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)")
	stmts["statusInsert"], _ = db.Prepare("INSERT INTO status VALUES ($1, $2, $3, $4)")

	db.QueryRow("INSERT INTO scrape DEFAULT VALUES RETURNING id").Scan(&scrapeid)

	categories := []int {201, 207}

	for _, category := range categories {
		for i := 0; i < 10; i++ {
			url := fmt.Sprintf("http://thepiratebay.se/browse/%d/%d/9/", category, i)
			log.Println(url)
			scrapePage(url)
		}
	}

	views := []string {
		"movieScrape",
		"top",
		"topMovie",
		"topMovieScrape",
	}
	for _, view := range views {
		db.Exec("REFRESH MATERIALIZED VIEW " + view)
	}
}