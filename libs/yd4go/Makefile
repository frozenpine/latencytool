NAME="yd4go"
VERSION=1.0.0
BUILD=`date +%FT%T%z`

PACKAGES=`go list ./... | grep -v /vendor/`
VETPACKAGES=`go list ./... | grep -v /vendor/ | grep -v /examples/`
GOFILES=`find . -name "*.go" -type f -not -path "./vendor/*"`
EXTRA_FLAGS=-X 'main.version=${VERSION}' \
			-X 'main.goVersion=`go version|cut -d ' ' -f3`' \
			-X 'main.gitVersion=`git rev-parse --short HEAD`' \
			-X 'main.buildTime=`date '+%Y-%m-%d %H:%M:%S'`'

ifeq ($(OS), Windows_NT)
	BINARY=${NAME}.dll
else
	BINARY=${NAME}.so
endif

default:
	@go build -buildmode=c-shared -ldflags "${EXTRA_FLAGS}" -o ${BINARY}

plugin:
	@go build -buildmode=plugin -ldflags "${EXTRA_FLAGS}" -o ${name}.plugin

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
	@if [ -f ${BINARY} ] ; then rm -f ${BINARY} ; fi

.PHONY: default fmt fmt-check install test vet docker clean