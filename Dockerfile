FROM golang:1.7-alpine

ADD . /go/src/bosun.org

WORKDIR /go/src/bosun.org
RUN go run /go/src/bosun.org/build/build.go

RUN rm -rf /go/src

RUN ls /go/bin

RUN bosun -version
