FROM golang:1.7-alpine

RUN apk update && apk upgrade && \
    apk add --no-cache bash git

ADD . /go/src/bosun.org

WORKDIR /go/src/bosun.org
RUN go run /go/src/bosun.org/build/build.go

RUN mkdir -p /bosun/config && cp /go/src/bosun.org/docker-compose/bosun.minimal.toml /bosun/config/bosun.toml
RUN touch /bosun/config/bosun.conf
RUN rm -rf /go/src

RUN ls /go/bin

RUN bosun -version

EXPOSE 8080

#in case you need direct access to ledis
EXPOSE 9565

#volume for ledisdb data
VOLUME /bosun/data

#volume for bosun config
VOLUME /bosun/config

CMD /go/bin/bosun -c /bosun/config/bosun.toml