// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.26.0
// source: queries.sql

package monitor_db

import (
	"context"
	"database/sql"
)

const getAllRequests = `-- name: GetAllRequests :many
select clientid, endpoint from requests order by clientId
`

func (q *Queries) GetAllRequests(ctx context.Context) ([]Request, error) {
	rows, err := q.db.QueryContext(ctx, getAllRequests)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Request
	for rows.Next() {
		var i Request
		if err := rows.Scan(&i.Clientid, &i.Endpoint); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getClientEndpointByIndex = `-- name: GetClientEndpointByIndex :one
select clientid, endpoint from requests where clientId = ? order by endpoint limit 1 offset ?
`

type GetClientEndpointByIndexParams struct {
	Clientid int64
	Offset   int64
}

func (q *Queries) GetClientEndpointByIndex(ctx context.Context, arg GetClientEndpointByIndexParams) (Request, error) {
	row := q.db.QueryRowContext(ctx, getClientEndpointByIndex, arg.Clientid, arg.Offset)
	var i Request
	err := row.Scan(&i.Clientid, &i.Endpoint)
	return i, err
}

const getClientEndpointsAmount = `-- name: GetClientEndpointsAmount :one
select count(*) from requests where clientId = ?
`

func (q *Queries) GetClientEndpointsAmount(ctx context.Context, clientid int64) (int64, error) {
	row := q.db.QueryRowContext(ctx, getClientEndpointsAmount, clientid)
	var count int64
	err := row.Scan(&count)
	return count, err
}

const getEndpointsToMonitor = `-- name: GetEndpointsToMonitor :many
select id, url from urls_to_request
`

func (q *Queries) GetEndpointsToMonitor(ctx context.Context) ([]UrlsToRequest, error) {
	rows, err := q.db.QueryContext(ctx, getEndpointsToMonitor)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []UrlsToRequest
	for rows.Next() {
		var i UrlsToRequest
		if err := rows.Scan(&i.ID, &i.Url); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getRequestsByClientId = `-- name: GetRequestsByClientId :many
select clientid, endpoint from requests where clientId = ? order by endpoint
`

func (q *Queries) GetRequestsByClientId(ctx context.Context, clientid int64) ([]Request, error) {
	rows, err := q.db.QueryContext(ctx, getRequestsByClientId, clientid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Request
	for rows.Next() {
		var i Request
		if err := rows.Scan(&i.Clientid, &i.Endpoint); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const getUsersToNotify = `-- name: GetUsersToNotify :many
select clients.clientId
    from user_url_subscription
        inner join clients on clients.clientId = user_url_subscription.clientId
    where user_url_subscription.urlId = ?
`

func (q *Queries) GetUsersToNotify(ctx context.Context, urlid sql.NullInt64) ([]int64, error) {
	rows, err := q.db.QueryContext(ctx, getUsersToNotify, urlid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []int64
	for rows.Next() {
		var clientid int64
		if err := rows.Scan(&clientid); err != nil {
			return nil, err
		}
		items = append(items, clientid)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const insertRequest = `-- name: InsertRequest :exec
insert into requests (clientId, endpoint) values (?, ?)
`

type InsertRequestParams struct {
	Clientid int64
	Endpoint string
}

func (q *Queries) InsertRequest(ctx context.Context, arg InsertRequestParams) error {
	_, err := q.db.ExecContext(ctx, insertRequest, arg.Clientid, arg.Endpoint)
	return err
}

const removeRequest = `-- name: RemoveRequest :exec
delete from requests where clientId = ? and endpoint = ?
`

type RemoveRequestParams struct {
	Clientid int64
	Endpoint string
}

func (q *Queries) RemoveRequest(ctx context.Context, arg RemoveRequestParams) error {
	_, err := q.db.ExecContext(ctx, removeRequest, arg.Clientid, arg.Endpoint)
	return err
}
