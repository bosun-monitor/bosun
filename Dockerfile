FROM golang:1.9.1

COPY . $GOPATH/src/bosun.org
WORKDIR ${GOPATH}/src/bosun.org/build
RUN go run build.go -esv5 -bosun

WORKDIR ${GOPATH}/src/bosun.org
RUN cd toml-merge && go install

COPY bosun.toml /bosun/bosun.toml