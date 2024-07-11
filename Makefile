build-docker-image:
	docker build -t "telegram-http-monitor:latest" .

run-docker-image:
	docker run -v ./config.yaml:/app/config.yaml telegram-http-monitor:latest

run:
	go run pafaul/telegram-http-monitor