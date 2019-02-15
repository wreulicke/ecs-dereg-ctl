ARCH := 386 amd64
OS := linux darwin windows

VERSION=$(shell ./version.sh)
VERSION_FLAG=\"main.version=${VERSION}\"

setup:
	GO111MODULE=off go get github.com/mitchellh/gox

build: 
	go build -ldflags "-X $(VERSION_FLAG)" -o ./dist/ecs-dereg-ctl .

build-all: 
	GO111MODULE=on gox -os="$(OS)" -arch="$(ARCH)" -ldflags "-X $(VERSION_FLAG)" -output "./dist/{{.Dir}}_{{.OS}}_{{.Arch}}" ./cmd/ecs-dereg-ctl
	
release: 
	GO111MODULE=off go get github.com/tcnksm/ghr
	@ghr -u $(CIRCLE_PROJECT_USERNAME) -r $(CIRCLE_PROJECT_REPONAME) $(VERSION) dist/