.PHONY: test

test:
	go test ./... -timeout 60s -v -cover -race