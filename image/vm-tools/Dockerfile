FROM ubuntu:18.04

COPY RancherVM-debootstrap-ubuntu-1804.tgz /opt/rancher/vm-tools/
COPY startvm /opt/rancher/vm-tools/
COPY start.sh /usr/bin/

CMD ["/usr/bin/start.sh"]
