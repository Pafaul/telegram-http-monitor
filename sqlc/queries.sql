-- name: InsertRequest :exec
insert into requests (clientId, endpoint) values (?, ?);

-- name: RemoveRequest :exec
delete from requests where clientId = ? and endpoint = ?;

-- name: GetRequestsByClientId :many
select * from requests where clientId = ? order by endpoint;

-- name: GetAllRequests :many
select * from requests order by clientId;

-- name: GetClientEndpointByIndex :one
select * from requests where clientId = ? order by endpoint limit 1 offset ?;

-- name: GetClientEndpointsAmount :one
select count(*) from requests where clientId = ?;

-- name: GetEndpointsToMonitor :many
select * from urls_to_request;

-- name: GetUsersToNotify :many
select clients.clientId
    from user_url_subscription
        inner join clients on clients.clientId = user_url_subscription.clientId
    where user_url_subscription.urlId = ?;

-- name: RemoveClient :exec
delete from clients where clientId = ?;