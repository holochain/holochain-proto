GOBIN = $(value GOPATH)/bin
.PHONY: test deps gx work publish
# Anything which requires deps should end with: gx-go rewrite --undo

hc: deps
	go install ./cmd/hc
	gx-go rewrite --undo
bs: deps
	go install ./cmd/bs
	gx-go rewrite --undo
test: deps
	go get -t
	go test -v ./...||exit 1
	gx-go rewrite --undo
$(GOBIN)/gx:
	go get -u github.com/whyrusleeping/gx
$(GOBIN)/gx-go:
	go get -u github.com/whyrusleeping/gx-go
gx: $(GOBIN)/gx $(GOBIN)/gx-go
	gx install --global
deps: gx
	gx-go rewrite
	go get -d ./...
