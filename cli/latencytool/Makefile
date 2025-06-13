NAME="latencytool"
VERSION=1.0.0
BUILD=`date +%FT%T%z`

PACKAGES=`go list ./... | grep -v /vendor/`
VETPACKAGES=`go list ./... | grep -v /vendor/ | grep -v /examples/`
GOFILES=`find . -name "*.go" -type f -not -path "./vendor/*"`
EXTRA_FLAGS=-X 'github.com/frozenpine/latency4go/cli/latencytool/cmd.version=${VERSION}' \
			-X 'github.com/frozenpine/latency4go/cli/latencytool/cmd.goVersion=`go version|cut -d ' ' -f3`' \
			-X 'github.com/frozenpine/latency4go/cli/latencytool/cmd.gitVersion=`git rev-parse --short HEAD`' \
			-X 'github.com/frozenpine/latency4go/cli/latencytool/cmd.buildTime=`date '+%Y-%m-%d %H:%M:%S'`'

ifeq ($(OS), Windows_NT)
	BINARY=${NAME}.exe
else
	BINARY=${NAME}
endif

default:
	@go build -ldflags "${EXTRA_FLAGS}" -o bin/${BINARY}

linux:
	@go build -ldflags "${EXTRA_FLAGS}" -o bin/${BINARY}

win32:
	@go build -ldflags "${EXTRA_FLAGS}" -o bin/${BINARY}

list:
	@echo ${PACKAGES}
	@echo ${GOFILES}

fmt:
	@gofmt -s -w ${GOFILES}

fmt-check:
	@diff=$$(gofmt -s -d $(GOFILES)); \
	if [ -n "$$diff" ]; then \
		echo "Please run 'make fmt' and commit the result:"; \
		echo "$${diff}"; \
		exit 1; \
	fi;

install:
	@govendor sync -v

test:
	@go test -cpu=1,2,4 -v -tags integration ./...

vet:
	@go vet $(VETPACKAGES)

# docker:
#     @docker build -t wuxiaoxiaoshen/example:latest .

clean:
	@if [ -f bin/${BINARY} ] ; then rm -f bin/${BINARY} ; fi

.PHONY: default fmt fmt-check install test vet docker clean