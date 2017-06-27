.PHONY: clean prepare api api/client build docker/push docker/build docker/clean run deps

ARCH ?= amd64
GOARCH ?= ${ARCH}
GOARM ?= 7

NAME=ddns
PKG=github.com/muka/${NAME}
GOPKGS=${GOPATH}/src
GOPKGSRC=${GOPKGS}/${PKG}
IMAGE="raptorbox/${NAME}-${ARCH}"
CGO ?= 0

clean:
	rm -rf ./build

prepare:
	mkdir -p ./build

deps:
	go get -u github.com/go-swagger/go-swagger/cmd/swagger

api:
	protoc -I/usr/local/include -I. -I${GOPATH}/src -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --go_out=google/api/annotations.proto=github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/google/api,plugins=grpc:. api/api.proto
	protoc -I/usr/local/include -I. -I${GOPATH}/src -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --grpc-gateway_out=logtostderr=true:. api/api.proto
	protoc -I/usr/local/include -I. -I${GOPATH}/src -I${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis --swagger_out=logtostderr=true:. api/api.proto

api/client: api
	swagger generate client -f api/api.swagger.json

build: prepare api api/client
	CGO_ENABLED=${CGO} ARCH=${ARCH} GOARCH=${GOARCH} GOARM=${GOARM} go build -o ./build/${NAME} cli/cli.go

run: api api/client
	go run cli/cli.go

docker/build:
	docker build . -t ${IMAGE}

docker/push: docker/build
	docker push ${IMAGE}

docker/clean:
	docker rmi $(docker images | grep ${NAME} | awk '{print $1}')

all: deps build api/client docker/build
