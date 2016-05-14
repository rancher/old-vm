FROM ubuntu:14.04.2
RUN apt-get update && \
    apt-get install -y qemu-kvm dnsmasq \
		       bridge-utils genisoimage curl jq
ADD https://github.com/rancher/vmnet/releases/download/v0.2.0/tapclient /usr/bin/
RUN chmod +x /usr/bin/tapclient
COPY startvm /var/lib/rancher/startvm
ENTRYPOINT ["/var/lib/rancher/startvm"]
VOLUME /image
EXPOSE 22
CMD []
