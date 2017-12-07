ifndef GOPATH
$(error GOPATH *must* be defined)
endif

GOBIN = $(value GOPATH)/bin

ifeq ($(OS),Windows_NT)
$(warning using hardcoded repo of github.com/metacurrency/holochain)
REPO = github.com/metacurrency/holochain
else
REPO = $(CURDIR:$(GOPATH)/src/%=%)
endif
# Remove a $(GOPATH)/src/ from the beginning of the current directory.
# Likely to be github.com/metacurrency/holochain

go_packages = . ./ui ./apptest $(sort $(dir $(wildcard ./cmd/*/)))
# List of directories containing go packages

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

TEST_FLAGS = -v

define new_line


endef

.PHONY: hcd hcdev hcadmin hccore bs test deps work pub
# Anything which requires deps should end with: gx-go rewrite --undo

all: deps
	$(foreach pkg_path,$(go_packages),go get $(pkg_path) ${new_line})
	gx-go rewrite --undo
hcd: deps
	go get $(REPO)/cmd/hcd
	gx-go rewrite --undo
hcdev: deps
	go get $(REPO)/cmd/hcdev
	gx-go rewrite --undo
hccore: deps
	go get $(REPO)/cmd/hccore
	gx-go rewrite --undo
hcadmin: deps
	go get $(REPO)/cmd/hcadmin
	gx-go rewrite --undo
bs: deps
	go get $(REPO)/cmd/bs
	gx-go rewrite --undo
test: deps
	$(foreach pkg_path,$(go_packages),go get -d -t $(pkg_path) && go test $(TEST_FLAGS) $(pkg_path)${new_line})
	gx-go rewrite --undo
deps: $(GOBIN)/gx $(GOBIN)/gx-go
	gx-go get $(REPO)
$(GOBIN)/gx:
	go get -u github.com/whyrusleeping/gx
$(GOBIN)/gx-go:
	go get -u github.com/whyrusleeping/gx-go
work: $(GOBIN)/gx-go
	gx-go rewrite
pub: $(GOBIN)/gx-go
	gx-go rewrite --undo
