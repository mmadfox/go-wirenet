
deps:
	go mod download

covertest: deps
	go test  -coverprofile=coverage.out
	go tool cover -html=coverage.out

mocks:
	mockgen -package=wirenet -destination=listener_mock.go -source=listener.go