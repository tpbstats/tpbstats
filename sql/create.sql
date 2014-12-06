drop schema public cascade;
create schema public;

CREATE TABLE scrape(
	id serial primary key,
	time timestamp default current_timestamp
);

CREATE TABLE movie(
	id int primary key not null,
	title varchar(100),
	released date,
	imdb_rating int,
	imdb_votes int,
	tomato_meter int,
	tomato_reviews int,
	tomato_user_meter int,
	tomato_user_reviews int,
	trailer varchar(100),
	scrape int not null references scrape(id)
);

CREATE TABLE torrent(
	id int primary key not null,
	name varchar(100) not null,
	uploaded timestamp not null,
	size bigint not null,
	movie int references movie(id),
	scrape int not null references scrape(id)
);

CREATE TABLE status(
	torrent int not null references torrent(id),
	seeders int not null,
	leechers int not null,
	scrape int not null references scrape(id),
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

CREATE VIEW movieNeedsUpdate AS
select movie.id
from movie, scrape
where
	movie.scrape = scrape.id
	AND scrape.time < now() - interval '7 days';