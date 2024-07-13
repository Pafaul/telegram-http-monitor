CREATE TABLE IF NOT EXISTS requests
(
    clientId int not null,
    endpoint text not null,
    primary key (clientId, endpoint)
);