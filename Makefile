.PHONY: all build build-web run test clean fmt vet pre-commit help

PROJECT = paopao-ce
TARGET = paopao
ifeq ($(OS),Windows_NT)
TARGET := $(TARGET).exe
endif
TARGET_BIN = $(basename $(TARGET))

ifeq (n$(CGO_ENABLED),n)
CGO_ENABLED := 1
endif

RELEASE_ROOT = release
RELEASE_FILES = LICENSE README.md CHANGELOG.md config.yaml.sample scripts docs
RELEASE_LINUX_AMD64 = $(RELEASE_ROOT)/linux-amd64/$(PROJECT)
RELEASE_DARWIN_AMD64 = $(RELEASE_ROOT)/darwin-amd64/$(PROJECT)
RELEASE_DARWIN_ARM64 = $(RELEASE_ROOT)/darwin-arm64/$(PROJECT)
RELEASE_WINDOWS_AMD64 = $(RELEASE_ROOT)/windows-amd64/$(PROJECT)

BUILD_VERSION := $(shell git describe --tags --always)
BUILD_DATE := $(shell date +'%Y-%m-%d %H:%M:%S %Z')
SHA_SHORT := $(shell git rev-parse --short HEAD)

GOFMT ?= gofumpt -l -w
MOD_NAME = github.com/BZYA-Community/WebsiteCore
LDFLAGS = -X "${MOD_NAME}/pkg/version.version=${BUILD_VERSION}" \
          -X "${MOD_NAME}/pkg/version.buildDate=${BUILD_DATE}" \
          -X "${MOD_NAME}/pkg/version.commitID=${SHA_SHORT}" \
          -X "${MOD_NAME}/pkg/version.buildTags=${TAGS}" \
		  -w -s

COMPOSE_DEV = docker compose -f docker-compose.dev.yml

all: fmt build

build:
	@echo Build paopao-ce
	@go build -pgo=auto -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_ROOT)/$(TARGET)

buildx:
	@go mod download
	@echo Build paopao-ce
	@go build -pgo=auto -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_ROOT)/$(TARGET)

build-web:
	@cd web && rm -rf dist/* && yarn build && cd -

run:
	@go run -pgo=auto -trimpath -gcflags "all=-N -l" -tags '$(TAGS)' -ldflags '$(LDFLAGS)' . serve

.PHONY: migrate
migrate:
	@echo "Migrating database..."
	@go run -trimpath -tags 'migration $(TAGS)' -ldflags '$(LDFLAGS)' . migrate

.PHONY: deps-up deps-down deps-logs deps-status deps-reset
deps-up:
	@echo "Starting dev dependency stack..."
	@$(COMPOSE_DEV) up -d --wait

deps-down:
	@echo "Stopping dev dependency stack (volumes kept)..."
	@$(COMPOSE_DEV) down

deps-logs:
	@$(COMPOSE_DEV) logs -f

deps-status:
	@$(COMPOSE_DEV) ps

deps-reset:
	@echo "Removing dev dependency stack and its volumes..."
	@$(COMPOSE_DEV) down -v

.PHONY: release
release: linux-amd64 darwin-amd64 darwin-arm64 windows-x64
	@echo Package paopao-ce
	@cp -rf $(RELEASE_FILES) $(RELEASE_LINUX_AMD64)
	@cp -rf $(RELEASE_FILES) $(RELEASE_DARWIN_AMD64)
	@cp -rf $(RELEASE_FILES) $(RELEASE_DARWIN_ARM64)
	@cp -rf $(RELEASE_FILES) $(RELEASE_WINDOWS_AMD64)
	@cd $(RELEASE_LINUX_AMD64)/.. && rm -f *.zip && zip -r $(PROJECT)-linux_amd64.zip $(PROJECT) && cd -
	@cd $(RELEASE_DARWIN_AMD64)/.. && rm -f *.zip && zip -r $(PROJECT)-darwin_amd64.zip $(PROJECT) && cd -
	@cd $(RELEASE_DARWIN_ARM64)/.. && rm -f *.zip && zip -r $(PROJECT)-darwin_arm64.zip $(PROJECT) && cd -
	@cd $(RELEASE_WINDOWS_AMD64)/.. && rm -f *.zip && zip -r $(PROJECT)-windows_amd64.zip $(PROJECT) && cd -

.PHONY: linux-amd64
linux-amd64:
	@echo Build paopao-ce [linux-amd64] CGO_ENABLED=$(CGO_ENABLED) TAGS="'$(TAGS)'"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build -pgo=auto -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_LINUX_AMD64)/$(TARGET_BIN)

.PHONY: darwin-amd64
darwin-amd64:
	@echo Build paopao-ce [darwin-amd64] CGO_ENABLED=$(CGO_ENABLED) TAGS="'$(TAGS)'"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=amd64 go build -pgo=auto -trimpath  -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_DARWIN_AMD64)/$(TARGET_BIN)

.PHONY: darwin-arm64
darwin-arm64:
	@echo Build paopao-ce [darwin-arm64] CGO_ENABLED=$(CGO_ENABLED) TAGS="'$(TAGS)'"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=darwin GOARCH=arm64 go build -pgo=auto -trimpath -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_DARWIN_ARM64)/$(TARGET_BIN)

.PHONY: windows-x64
windows-x64:
	@echo Build paopao-ce [windows-x64] CGO_ENABLED=$(CGO_ENABLED) TAGS="'$(TAGS)'"
	@CGO_ENABLED=$(CGO_ENABLED) GOOS=windows GOARCH=amd64 go build -pgo=auto -trimpath  -tags '$(TAGS)' -ldflags '$(LDFLAGS)' -o $(RELEASE_WINDOWS_AMD64)/$(TARGET_BIN).exe

.PHONY: generate
generate: gen-mir gen-enum

.PHONY: gen-mir
gen-mir:
	@go generate mirc/gen.go
	$(GOFMT) ./auto/api

.PHONY: gen-enum
gen-enum:
	@go generate ./internal/model/enum/...
	$(GOFMT) ./internal/model/enum

clean:
	@go clean
	@find ./release -type f -exec rm -r {} +

fmt:
	@echo Formatting...
	$(GOFMT) .

vet:
	@go vet -composites=false ./internal/...
	@go vet -composites=false ./pkg/...

test:
	@go test ./...

pre-commit: fmt
	@go mod tidy

.PHONY: install-tools
install-tools:
	@go install github.com/abice/go-enum@latest
	@go install mvdan.cc/gofumpt@latest

help:
	@echo "make: make"
	@echo "make run: start api server"
	@echo "make build: build executable"
	@echo "make release: build release executables"
	@echo "make run TAGS='embed': start api server and serve embed web frontend"
	@echo "make build TAGS='embed': build executable with embed web frontend"
	@echo "make release TAGS='embed': build release executables with embed web frontend"
	@echo "make migrate: run database migrations"
	@echo "make deps-up: start dev dependency stack"
	@echo "make deps-down: stop dev dependency stack, keeping data volumes"
	@echo "make deps-logs: follow dev dependency stack logs"
	@echo "make deps-status: show dev dependency stack status"
	@echo "make deps-reset: remove dev dependency stack and its data volumes"
