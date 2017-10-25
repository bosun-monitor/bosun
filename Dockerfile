FROM golang:1.9

COPY . $GOPATH/src/bosun.org
WORKDIR ${GOPATH}/src/bosun.org/build
RUN go run build.go -esv5 -bosun
