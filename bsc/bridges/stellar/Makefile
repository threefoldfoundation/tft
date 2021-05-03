ldflags = '-w -s -extldflags "-static"'

build:
	GOOS=linux go build -tags netgo -ldflags $(ldflags)

docker: build
	cp -r stellar genesis docker/ && cd docker && docker build -t eth-bridge:latest .