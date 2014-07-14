FROM ubuntu:latest
MAINTAINER Peter Grace, pete@stackexchange.com
RUN apt-get update
RUN apt-get install -y supervisor openssh-server unzip git
ADD docker/supervisor-system.conf /etc/supervisor/conf.d/system.conf

#SSH
ADD docker/supervisor-sshd.conf /etc/supervisor/conf.d/sshd.conf
RUN mkdir -p /root/.ssh
RUN chmod 0600 /root/.ssh
RUN sed -ri 's/UsePAM yes/#UsePAM yes/g; s/#UsePAM no/UsePAM no/g;' /etc/ssh/sshd_config
RUN mkdir -p /var/run/sshd
RUN chown 0:0 /var/run/sshd
RUN chmod 0744 /var/run/sshd
ADD docker/create_ssh_key.sh /opt/sei-bin/

#Serf
ADD docker/supervisor-serf.conf /etc/supervisor/conf.d/serf.conf
ADD https://dl.bintray.com/mitchellh/serf/0.6.1_linux_amd64.zip /opt/downloads/
WORKDIR /opt/downloads
RUN ["/bin/bash","-c","unzip 0.6.1_linux_amd64.zip"]
RUN mv /opt/downloads/serf /usr/bin
ADD docker/serf-start.sh /opt/sei-bin/
ADD docker/serf-join.sh /opt/sei-bin/

#Go
ADD http://golang.org/dl/go1.3.linux-amd64.tar.gz /opt/downloads/
RUN tar -C /usr/local -xzf go1.3.linux-amd64.tar.gz

#Bosun
ADD docker/supervisor-bosun.conf /etc/supervisor/conf.d/bosun.conf
ADD . /opt/bosun
WORKDIR /opt/bosun
RUN mkdir -p /opt/bosun/state
ADD docker/docker.conf /opt/bosun/
RUN mkdir -p /opt/bosun/src/github.com/StackExchange
RUN ln -s /opt/bosun /opt/bosun/src/github.com/StackExchange/bosun
RUN GOPATH=/opt/bosun /usr/local/go/bin/go build .
CMD ["/usr/bin/supervisord"]
EXPOSE 4242
EXPOSE 8070
