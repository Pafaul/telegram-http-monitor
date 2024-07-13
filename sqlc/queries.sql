-- name: InsertRequest :exec
insert into requests (clientId, endpoint) values (?, ?);

-- name: RemoveRequest :exec
delete from requests where clientId = ? and endpoint = ?;

-- name: GetRequestsByClientId :many
select * from requests where clientId = ?;

-- name: GetAllRequests :many
select * from requests;
