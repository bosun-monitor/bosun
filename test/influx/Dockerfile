FROM debian:wheezy

RUN apt-get update && apt-get install -y \
	curl \
	--no-install-recommends \
	&& rm -rf /var/lib/apt/lists/*

ENV INFLUXVER 0.9.0-rc24

RUN curl http://get.influxdb.org/influxdb_${INFLUXVER}_amd64.deb > /tmp/deb \
	&& dpkg -i /tmp/deb \
	&& rm /tmp/deb

EXPOSE 8083 8086
CMD ["/opt/influxdb/influxd", "-config", "/etc/opt/influxdb/influxdb.conf"]