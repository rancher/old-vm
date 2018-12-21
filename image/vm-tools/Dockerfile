FROM ubuntu:18.04

RUN \
    apt-get update && \
    apt-get install -y qemu-kvm qemu-utils bridge-utils genisoimage curl net-tools && \
    apt-get autoclean && \
    apt-get autoremove && \
    rm -rf /var/lib/apt/lists/*

COPY RancherVM-debootstrap-ubuntu-1804.tgz /opt/rancher/vm-tools/
COPY startvm /opt/rancher/vm-tools/
COPY start.sh /usr/bin/

CMD ["/usr/bin/start.sh"]
