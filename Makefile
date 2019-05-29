.PHONY: clean prepare api api/client build docker/push docker/build docker/clean run deps

ARCH ?= amd64
GOARCH ?= ${ARCH}
GOARM ?= 7

NAME=ddns
PKG=github.com/muka/${NAME}
GOPKGS=${GOPATH}/src
GOPKGSRC=${GOPKGS}/${PKG}
IMAGE="opny/${NAME}-${ARCH}"
CGO ?= 0

PROTOC := ./data/protoc/bin/protoc \
	-I ./data/protoc/include \
	-I /usr/local/include \
	-I. \
	-I ${GOPATH}/src \
	-I ${GOPATH}/src/github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis


setup: setup/protoc setup/godeps

setup/protoc:
	mkdir -p ./data/protoc
	cd data/protoc && \
		wget https://github.com/protocolbuffers/protobuf/releases/download/v3.8.0/protoc-3.8.0-linux-x86_64.zip && \
		unzip protoc-3.8.0-linux-x86_64.zip && \
		rm -f protoc-3.8.0-linux-x86_64.zip

setup/godeps:
	go get -u -v ./...

clean:
	rm -rf ./build

prepare:
	mkdir -p ./build

deps:
	go get -u google.golang.org/grpc
	go get -u github.com/golang/protobuf/proto
	go get -u github.com/golang/protobuf/protoc-gen-go
	go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-grpc-gateway
	go get -u github.com/grpc-ecosystem/grpc-gateway/protoc-gen-swagger

api:
	${PROTOC} --go_out=google/api/annotations.proto=github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/google/api,plugins=grpc:. api/coredns.proto
	${PROTOC} --go_out=google/api/annotations.proto=github.com/grpc-ecosystem/grpc-gateway/third_party/googleapis/google/api,plugins=grpc:. api/api.proto
	${PROTOC} --grpc-gateway_out=logtostderr=true:. api/api.proto
	${PROTOC} --swagger_out=logtostderr=true:. api/api.proto

build: prepare api
	CGO_ENABLED=${CGO} ARCH=${ARCH} GOARCH=${GOARCH} GOARM=${GOARM} go build -o ./build/${NAME} cli/cli.go

run: api api/client
	go run cli/cli.go

docker/build:
	docker build . -t ${IMAGE}

docker/push: docker/build
	docker push ${IMAGE}

docker/clean:
	docker rmi $(docker images | grep ${NAME} | awk '{print $1}')

all: build api/client docker/build
