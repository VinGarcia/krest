args=
path=./...

GOBIN=$(shell go env GOPATH)/bin

test: setup
	$(GOBIN)/richgo test $(path) $(args)

lint: setup
	@$(GOBIN)/staticcheck $(path) $(args)
	@go vet $(path) $(args)
	@$(GOBIN)/errcheck ./...
	@echo "StaticCheck & Go Vet & ErrCheck found no problems on your code!"

setup: $(GOBIN)/richgo $(GOBIN)/staticcheck $(GOBIN)/errcheck

$(GOBIN)/richgo:
	go get github.com/kyoh86/richgo

$(GOBIN)/staticcheck:
	go install honnef.co/go/tools/cmd/staticcheck@latest

$(GOBIN)/errcheck:
	go install github.com/kisielk/errcheck@latest
