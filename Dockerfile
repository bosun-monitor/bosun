FROM golang:1.7-alpine

RUN apt-get update && apt-get install -y \
    git \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*
    
ADD . /go/src/bosun.org

WORKDIR /go/src/bosun.org
RUN go run /go/src/bosun.org/build/build.go

RUN rm -rf /go/src

RUN ls /go/bin

RUN bosun -version
