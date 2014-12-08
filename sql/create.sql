drop schema public cascade;
create schema public;

CREATE TABLE scrape(
	id serial primary key,
	time timestamp default current_timestamp
);

CREATE TABLE movie(
	id int primary key NOT NULL,
	title varchar(100),
	released date,
	imdb_rating int,
	imdb_votes int,
	tomato_meter int,
	tomato_reviews int,
	tomato_user_meter int,
	tomato_user_reviews int,
	trailer varchar(100),
	scrape int NOT NULL REFERENCES scrape(id)
);

CREATE TABLE torrent(
	id int primary key NOT NULL,
	name varchar(100) NOT NULL,
	uploaded timestamp NOT NULL,
	size bigint NOT NULL,
	movie int REFERENCES movie(id),
	scrape int NOT NULL REFERENCES scrape(id)
);

CREATE TABLE status(
	torrent int NOT NULL REFERENCES torrent(id),
	seeders int NOT NULL,
	leechers int NOT NULL,
	scrape int NOT NULL REFERENCES scrape(id),
	primary key(torrent, scrape)
);

CREATE VIEW movieScrape AS
SELECT
	movie.id,
	status.scrape,
	SUM(seeders) seeders,
	SUM(leechers) leechers,
	SUM(seeders + leechers) peers
FROM movie, torrent, status, scrape
WHERE
	torrent.movie = movie.id
	AND status.torrent = torrent.id
	AND status.scrape = scrape.id
	AND scrape.time >  now() - interval '24 hours'
GROUP BY movie.id, status.scrape
ORDER BY status.scrape;

CREATE VIEW movieScrapeYesterday AS
SELECT
	movie.id,
	status.scrape,
	SUM(seeders) seeders,
	SUM(leechers) leechers,
	SUM(seeders + leechers) peers
FROM movie, torrent, status, scrape
WHERE
	torrent.movie = movie.id
	AND status.torrent = torrent.id
	AND status.scrape = scrape.id
	AND scrape.time < now() - interval '24 hours'
	AND scrape.time > now() - interval '48 hours'
GROUP BY movie.id, status.scrape
ORDER BY status.scrape;

CREATE VIEW top AS
SELECT 
	id,
	ROUND(AVG(seeders)) seeders,
	ROUND(AVG(leechers)) leechers,
	ROUND(AVG(peers)) peers
FROM movieScrape
GROUP BY id
ORDER BY peers DESC
LIMIT 10;

CREATE VIEW topMovie AS
SELECT *
FROM top
NATURAL JOIN movie
NATURAL JOIN deltaYesterday
ORDER BY top.peers DESC;

CREATE VIEW topMovieScrape AS
SELECT
	top.id,
	ms.scrape,
	ms.seeders,
	ms.leechers,
	ms.peers
FROM top, movieScrape ms
WHERE
	top.id = ms.id
ORDER BY top.peers DESC, ms.scrape ASC;

CREATE VIEW movieLastScrape AS
SELECT DISTINCT(torrent.movie) id
FROM status, torrent
WHERE
	status.torrent = torrent.id
	AND status.scrape = (SELECT last_value FROM scrape_id_seq);

CREATE VIEW movieNeedsUpdate AS
SELECT movie.id
FROM movie, scrape, movieLastScrape mls
WHERE
	movie.scrape = scrape.id
	AND movie.id = mls.id
	AND scrape.time < now() - interval '7 days'
LIMIT 10;

CREATE VIEW deltaYesterday AS
SELECT
	ms.id,
	ROUND(AVG(msy.peers)) yesterday,
	ROUND(AVG(ms.peers)) peers,
	ROUND(AVG(ms.peers) - AVG(msy.peers)) delta
FROM movieScrape ms
LEFT JOIN movieScrapeYesterday msy ON msy.id = ms.id
WHERE msy.peers IS NOT NULL
GROUP BY ms.id;

CREATE VIEW risingMovie AS
SELECT *
FROM deltaYesterday
NATURAL JOIN movie
ORDER BY delta DESC
LIMIT 10;

CREATE VIEW fallingMovie AS
SELECT *
FROM deltaYesterday
NATURAL JOIN movie
ORDER BY delta ASC
LIMIT 10;

CREATE VIEW new AS
SELECT
	ms.id, 
	ROUND(AVG(ms.peers)) peers
FROM movieScrape ms
LEFT JOIN movieScrapeYesterday msy ON msy.id = ms.id
WHERE msy.id IS NULL
GROUP BY ms.id
ORDER BY peers DESC
LIMIT 10;

CREATE VIEW newMovie AS
SELECT *
FROM new
NATURAL JOIN movie
ORDER BY peers DESC;