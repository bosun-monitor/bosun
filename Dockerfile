FROM debian:wheezy

RUN apt-get update && apt-get install -y \
    git \
    curl \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

RUN curl -SL https://storage.googleapis.com/golang/go1.7.linux-amd64.tar.gz \
    | tar -xzC /usr/local

ADD . /go/src/bosun.org

WORKDIR /go/src/bosun.org
RUN go run /go/src/bosun.org/build/build.go

RUN rm -rf /go/src

RUN ls /go/bin

RUN bosun -version
