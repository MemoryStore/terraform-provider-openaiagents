default: fmt lint install generate

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	env -u OPENAI_API_KEY TF_ACC=1 go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	@echo "Default tests are offline (fake API). Live acceptance is OPENAIAGENTS_ACC_LIVE=1 and is not invoked here."
	env -u OPENAI_API_KEY TF_ACC=1 go test -v -cover -timeout 120m ./...

.PHONY: fmt lint test testacc build install generate
