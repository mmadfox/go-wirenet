
deps:
	go mod download

covertest: deps
	go test  -coverprofile=coverage.out
	go tool cover -html=coverage.out

mocks:
	mockgen -package=wirenet -destination=hub_mock.go -source=hub.go