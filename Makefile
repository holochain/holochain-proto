test: deps
	go test -v ./...

gx:
	go get -u github.com/whyrusleeping/gx
	go get -u github.com/whyrusleeping/gx-go

install:
	gx --verbose install --global

deps: gx install work

work:
	gx-go rewrite

publish:
	gx-go rewrite --undo
