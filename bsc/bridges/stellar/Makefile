ldflags = '-w -s -extldflags "-static"'

build:
	GOOS=linux go build -tags netgo -ldflags $(ldflags)

docker: build
	docker build -t eth-bridge:latest .