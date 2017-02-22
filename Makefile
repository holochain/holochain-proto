all: deps hc

hc: deps
	go install ./cmd/hc

test: deps
	go test -v ./...

gx:
	go get -u github.com/whyrusleeping/gx
	go get -u github.com/whyrusleeping/gx-go

gxinstall:
	gx --verbose install --global

deps: gx gxinstall work
	go get -d ./..

work:
	gx-go rewrite

publish:
	gx-go rewrite --undo
