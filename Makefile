GOBIN = $(value GOPATH)/bin
.PHONY: test deps gx work publish
hc: deps
	go install ./cmd/hc
bs: deps
	go install ./cmd/bs
test: deps
	go get -t
	go test -v ./...||exit 1
$(GOBIN)/gx:
	go get -u github.com/whyrusleeping/gx
$(GOBIN)/gx-go:
	go get -u github.com/whyrusleeping/gx-go
gx: $(GOBIN)/gx $(GOBIN)/gx-go
	gx install --global
deps: gx
	gx-go rewrite
	go get -d ./...
work: $(GOBIN)/gx-go
	gx-go rewrite
publish: $(GOBIN)/gx-go
	gx-go rewrite --undo
