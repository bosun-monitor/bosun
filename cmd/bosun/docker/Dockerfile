FROM golang:1.13 AS bosun_builder

WORKDIR /bosun
COPY . /bosun

RUN make bosun scollector tsdbrelay

FROM alpine:latest

ENV PACKAGES "ca-certificates rsyslog bash libc6-compat curl libgd libpng libjpeg libwebp libjpeg-turbo cairo pango lua supervisor asciidoctor"

ENV BOSUN_ROOT "cmd/bosun/docker"

ENV DATA_DIR /data

ENV TERM xterm
ENV TSDBRELAY_OPTS -b localhost:8070 -t opentsdb:4242 -l 0.0.0.0:5252 -redis localhost:9565

# Install dependencies
RUN apk --update add apk-tools \
    && apk add ${PACKAGES}

# Copy Bosun from the build image
WORKDIR /bosun
RUN mkdir /scollector /tsdbrelay
COPY --from=bosun_builder /bosun/bosun /bosun
COPY --from=bosun_builder /bosun/scollector /scollector
COPY --from=bosun_builder /bosun/tsdbrelay /tsdbrelay

# Copy Bosun config
COPY ${BOSUN_ROOT}/data/bosunrules.conf ${DATA_DIR}/
COPY ${BOSUN_ROOT}/data/bosun.toml ${DATA_DIR}/
COPY ${BOSUN_ROOT}/data/scollector.toml ${DATA_DIR}/

# Copy supervisor config
COPY ${BOSUN_ROOT}/data/supervisord.conf ${DATA_DIR}/

EXPOSE 8070 5252 9565
VOLUME ["${DATA_DIR}"]
CMD ["sh", "-c", "/usr/bin/supervisord -c ${DATA_DIR}/supervisord.conf"]
