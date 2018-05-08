FROM ubuntu:16.04

COPY RancherVM-debootstrap-ubuntu-1604.tgz /opt/rancher/vm-tools/
COPY startvm /opt/rancher/vm-tools/
COPY start.sh /usr/bin/

CMD ["/usr/bin/start.sh"]
