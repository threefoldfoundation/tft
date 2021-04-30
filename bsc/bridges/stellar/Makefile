ldflags = '-w -s -extldflags "-static"'

build:
	GOOS=linux go build -tags netgo -ldflags $(ldflags)

docker: build
	cp -r stellar genesis docker/ && cd docker && docker build -t eth-bridge:latest .

generate-token:
	abigen --abi abis/tokenabi.json --pkg contract --type Token --out ./api/bridge/contract/token.go 

generate-mscontract:
	abigen --abi abis/mscontract.json --pkg mscontract --type Token --out ./api/bridge/mscontract/multisig.go 