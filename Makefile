# Version comes from the latest git tag (vX.Y.Z); commits after it read
# X.Y.Z-N-g<sha>. Release = `git tag vX.Y.Z && git push --tags`.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X hekato-go/config.Version=$(VERSION)

.PHONY: build linux dashboard

build:
	go build -ldflags "$(LDFLAGS)" -o hekato-go .

linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o hekato-go-linux-amd64 .

dashboard:
	cd dashboard && npm run build
