FROM golang:1.8

WORKDIR /go/src/bosun.org
COPY . .
RUN ls -la
RUN go install bosun.org/cmd/bosun
RUN ls /go/bin