build-docker-image:
	docker build -t "telegram-http-monitor:latest" .

run-docker-image:
	docker run -v ./config.yaml:/app/config.yaml telegram-http-monitor:latest

build:
	go build -race pafaul/telegram-http-monitor

run:
	go run pafaul/telegram-http-monitor

gen-db:
	sqlc generate

migrate-up:
	migrate -source file://sqlc/migrations -database sqlite3://db.sqlite up

migrate-down:
	migrate -source file://sqlc/migrations -database sqlite3://db.sqlite down
