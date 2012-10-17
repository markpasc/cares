INSERT INTO author (id, name, url)
	SELECT 1, displayName, '/' FROM account WHERE id = 1;

UPDATE post SET author = 1 WHERE author IS NULL;

ALTER TABLE post ALTER COLUMN author SET NOT NULL;
