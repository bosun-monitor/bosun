FROM golang:1.9

COPY . $GOPATH/src/bosun.org
WORKDIR ${GOPATH}/src/bosun.org/cmd/bosun
RUN go build
RUN go test bosun.org/...