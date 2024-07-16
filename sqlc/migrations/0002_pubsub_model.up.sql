CREATE TABLE urls_to_request (
    id INTEGER PRIMARY KEY,
    url TEXT NOT NULL UNIQUE
);

CREATE TABLE clients(
    clientId INTEGER PRIMARY KEY
);

CREATE TABLE user_url_subscription(
    id INTEGER PRIMARY KEY,
    clientId INTEGER,
    urlId INTEGER,
    FOREIGN KEY (clientId) REFERENCES clients(clientId),
    FOREIGN KEY (urlId) REFERENCES urls_to_request(id)
);