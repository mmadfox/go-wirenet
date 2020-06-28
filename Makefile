
deps:
	go mod download

covertest: deps
	go test  -coverprofile=coverage.out
	go tool cover -html=coverage.out

sync-coveralls: deps
	go test  -coverprofile=coverage.out `go list ./... | grep -v test`
	goveralls -coverprofile=coverage.out -reponame=go-wirenet -repotoken=${COVERALLS_GO_WIRENET_TOKEN} -service=local

proto:
	@protoc --go_out=. ./pb/session.proto