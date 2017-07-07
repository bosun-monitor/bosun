FROM golang:1.8

WORKDIR /go/src/bosun.org
COPY . .
RUN go install bosun.org/cmd/bosun
RUN ls /go/bin