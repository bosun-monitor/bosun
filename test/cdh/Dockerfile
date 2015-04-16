FROM centos:centos6

#RUN curl http://archive.cloudera.com/cdh4/one-click-install/redhat/6/x86_64/cloudera-cdh-4-0.x86_64.rpm > /tmp/cdh.rpm \
#	&& yum --nogpgcheck localinstall /tmp/cdh.rpm

RUN curl http://archive.cloudera.com/cdh4/redhat/6/x86_64/cdh/cloudera-cdh4.repo > /etc/yum.repos.d/cloudera-cdh4.repo \
	&& curl http://archive.cloudera.com/gplextras/redhat/6/x86_64/gplextras/cloudera-gplextras4.repo > /etc/yum.repos.d/cloudera-gplextras4.repo \
	&& yum check-update || true

RUN yum install -y hbase hadoop-lzo-cdh4
RUN yum install -y hbase-master

RUN yum install -y \
	automake \
	git \
	gnuplot \
	java-1.7.0-openjdk-devel \
	make \
	python-setuptools

RUN git clone -b next --depth 1 git://github.com/OpenTSDB/opentsdb.git /tsdb && \
	cd /tsdb && \
	bash ./build.sh

RUN easy_install supervisor

COPY opentsdb.conf /tsdb/
COPY start_opentsdb.sh /tsdb/
COPY supervisord.conf /etc/supervisord.conf
COPY create_tsdb_tables.sh /tsdb/

EXPOSE 4242
CMD ["supervisord"]
