FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	curl \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

RUN curl https://s3.amazonaws.com/influxdb/influxdb_0.10.0-1_amd64.deb > /tmp/deb \
	&& dpkg -i /tmp/deb \
	&& rm /tmp/deb

EXPOSE 8083 8086
CMD ["/opt/influxdb/influxd", "-config", "/etc/opt/influxdb/influxdb.conf"]
