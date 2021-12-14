args=
path=./...

GOBIN=$(shell go env GOPATH)/bin

test: setup
	$(GOBIN)/richgo test $(path) $(args)

lint: setup
	@go vet $(path) $(args)
	@$(GOBIN)/golint -set_exit_status -min_confidence 0.9 $(path) $(args)
	@echo "Golint & Go Vet found no problems on your code!"

setup: $(GOBIN)/richgo $(GOBIN)/golint

$(GOBIN)/richgo:
	go get github.com/kyoh86/richgo

$(GOBIN)/golint:
	go get golang.org/x/lint
