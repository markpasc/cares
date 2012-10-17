ALTER TABLE account ADD PRIMARY KEY (id);
ALTER TABLE post ADD PRIMARY KEY (id);
ALTER TABLE author ADD PRIMARY KEY (id);

CREATE TABLE writestream (
	id SERIAL PRIMARY KEY,
	account INTEGER NOT NULL REFERENCES account(id),
	post INTEGER NOT NULL REFERENCES post(id),
	posted TIMESTAMP WITH TIME ZONE NOT NULL
);

INSERT INTO writestream (account, post, posted)
	SELECT 1, id, posted FROM post WHERE author IS NULL;
