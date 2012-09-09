CREATE TABLE account (
	id SERIAL,
	name VARCHAR(30) UNIQUE NOT NULL,
	passwordHash VARCHAR(60) NOT NULL,
	displayName CHARACTER VARYING NOT NULL
);

CREATE TABLE post (
	id SERIAL,
	html CHARACTER VARYING NOT NULL,
	posted TIMESTAMP WITH TIME ZONE NOT NULL,
	created TIMESTAMP NOT NULL DEFAULT NOW(),
	deleted TIMESTAMP
);

CREATE TABLE rsscloud (
	id SERIAL,
	url VARCHAR(1024) UNIQUE NOT NULL,
	method VARCHAR(100) NOT NULL,
	subscribedUntil TIMESTAMP NOT NULL,
	created TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE schema (
	version INTEGER UNIQUE NOT NULL,
	upgraded TIMESTAMP NOT NULL DEFAULT NOW()
);