FROM ubuntu:14.04.2
RUN apt-get update && \
    apt-get install -y qemu-kvm dnsmasq \
		       bridge-utils genisoimage curl jq
COPY startvm /var/lib/rancher/startvm
COPY start.sh /usr/bin/start.sh
VOLUME /rancher
CMD ['start.sh']
