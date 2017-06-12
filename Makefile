GOBIN = $(value GOPATH)/bin

ifndef HOME
# Is probably a windows machine
ifdef USERPROFILE
HOME = $(USERPROFILE)
# Windows variable for home is USERPROFILE
else
$(error unable to get home directory)
endif
endif

HOLOPATH ?= $(HOME)/.holochain
# Default .holochain location

.PHONY: test test-sample deps gx work pub
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
test-sample: hc
# Init if not already init-ed
ifeq '$(and \
$(strip $(wildcard $(HOLOPATH))),\
$(strip $(wildcard $(HOLOPATH)/system.conf)),\
$(strip $(wildcard $(HOLOPATH)/agent.txt)))' ''
# If they all existed, the output of $(and) would be != ''
	$(warning hc not init-ed. using bogus email.)
	hc init node@example.com
endif
	hc --debug --verbose clone --force examples/sample examples-sample
	hc --debug --verbose test examples-sample
deps: gx
	gx-go rewrite
	go get -d ./...
gx: $(GOBIN)/gx $(GOBIN)/gx-go
	gx install --global
$(GOBIN)/gx:
	go get -u github.com/whyrusleeping/gx
$(GOBIN)/gx-go:
	go get -u github.com/whyrusleeping/gx-go
work: $(GOBIN)/gx-go
	gx-go rewrite
pub: $(GOBIN)/gx-go
	gx-go rewrite --undo
