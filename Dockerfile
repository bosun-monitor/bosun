FROM golang:1.9

COPY . $GOPATH/src/bosun.org
WORKDIR ${GOPATH}/src/bosun.org/build
RUN go run build.go -esv5 -bosun

RUN mkdir -p ${GOPATH}/src/gitlab.skyscannertools.net/data-platform
WORKDIR ${GOPATH}/src/gitlab.skyscannertools.net/data-platform
RUN git clone git@gitlab.skyscannertools.net:data-platform/toml-merge.git toml-merge
RUN go get -u gitlab.skyscannertools.net/data-platform/toml-merge

COPY bosun.toml /bosun/bosun.toml