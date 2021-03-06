cmd := $(shell ls cmd/)

all: init lint $(cmd) deinit

GitHash := github.com/dearcode/sapper/util.GitHash
GitTime := github.com/dearcode/sapper/util.GitTime
GitMessage := github.com/dearcode/sapper/util.GitMessage


LDFLAGS += -X "$(GitHash)=$(shell git log --pretty=format:'%H' -1)"
LDFLAGS += -X "$(GitTime)=$(shell git log --pretty=format:'%cd' -1)"
LDFLAGS += -X "$(GitMessage)=$(shell git log --pretty=format:'%cn %s %b' -1)"

files := $$(find . -name '*.go' | grep -vE 'vendor')
source := $(shell ls -ld */|awk '$$NF !~ /bin\/|logs\/|config\/|_vendor\/|vendor\/|web\/|docs\// {printf $$NF" "}')

golint:
	go get github.com/golang/lint/golint

init:
	if [[ -d _vendor ]]; then mv _vendor vendor; fi

deinit:
	if [[ -d vendor ]]; then mv vendor _vendor; fi

lint: golint
	for path in $(source); do golint "$$path..."; done;
	for path in $(source); do gofmt -s -l -w $$path;  done;
	go tool vet $(files) 2>&1
	go tool vet --shadow $(files) 2>&1

clean:
	@rm -rf bin

.PHONY: $(cmd)

$(cmd):
	go build -o bin/$@ -ldflags '$(LDFLAGS)' cmd/$@/main.go 


test:
	@for path in $(source); do echo "go test ./$$path"; go test "./"$$path; done;

