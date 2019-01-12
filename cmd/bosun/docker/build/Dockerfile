FROM debian:wheezy

RUN apt-get update && apt-get install -y \
    automake \
    curl \
    git \
    make \
    openjdk-7-jdk \
    python \
    --no-install-recommends \
    && rm -rf /var/lib/apt/lists/*

ENV TSDB /tsdb
RUN git clone --single-branch --branch v2.2.1 --depth 1 git://github.com/OpenTSDB/opentsdb.git $TSDB && \
    cd $TSDB && bash ./build.sh

ENV GOPATH /go
ENV HBASEVER 1.2.4
ENV HBASE /hbase/hbase-$HBASEVER
ENV JAVA_HOME /usr/lib/jvm/java-7-openjdk-amd64

RUN mkdir -p /hbase \
    && curl -SL http://archive.apache.org/dist/hbase/$HBASEVER/hbase-$HBASEVER-bin.tar.gz \
    | tar -xzC /hbase \
    && mv /hbase/hbase-$HBASEVER /hbase/hbase

RUN curl -SL https://storage.googleapis.com/golang/go1.11.linux-amd64.tar.gz \
    | tar -xzC /usr/local

COPY bosun $GOPATH/src/bosun.org/
WORKDIR $GOPATH/src/bosun.org

ENV PATH $PATH:/usr/local/go/bin:$GOPATH/bin

RUN go run build/build.go \
    && bosun -version \
    && scollector -version \
    && tsdbrelay -version
