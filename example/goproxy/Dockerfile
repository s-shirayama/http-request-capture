FROM golang:1.13rc1

RUN mkdir -p /go/src/main/ \
 && go get github.com/elazarl/goproxy
COPY main.go /go/src/main/
WORKDIR /go/src/main/

CMD go run main.go
