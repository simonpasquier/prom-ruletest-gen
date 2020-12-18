.PHONY: build
build: test format
	go build -tags netgo .

.PHONY: format
format:
	go fmt ./...

.PHONY: test
test:
	go test -race ./...

.PHONY: docker
docker: build
	docker build -t prom-ruletest-gen:latest .

