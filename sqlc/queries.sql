-- name: AddClient :exec
insert into clients(clientId) values(?) on conflict do nothing;

-- name: RemoveClient :exec
delete from clients where clientId = ?;

-- name: GetUrlIdToTrack :one
select id from urls_to_request where url = ?;

-- name: AddUrlToTrack :one
insert into urls_to_request(url) values (?) returning id;

-- name: RemoveUrlToTrack :exec
delete from urls_to_request where url = ?;

-- name: GetEndpointsToMonitor :many
select * from urls_to_request;

-- name: GetUsersToNotify :many
select c.clientId
from clients c
inner join user_url_subscription uus on c.clientId = uus.clientId
inner join urls_to_request ur on uus.urlId = ur.id
where ur.url = ?;

-- name: GetUserMonitoredEndpoints :many
select ur.url
from urls_to_request ur
inner join user_url_subscription uus on ur.id = uus.urlId
inner join clients c on uus.clientId = c.clientId
where c.clientId = ?
order by uus.id;

-- name: AddSubscription :exec
insert into user_url_subscription (clientId, urlId) values (?, ?);

-- name: RemoveSubscription :exec
delete from user_url_subscription where clientId = ? and urlId = ?;