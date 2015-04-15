FROM ubuntu:14.04
RUN apt-get update && apt-get install -y dnsmasq python-minimal busybox
ADD https://bootstrap.pypa.io/get-pip.py /usr/bin/
RUN python /usr/bin/get-pip.py
RUN pip install docker-py

ENV VNC noVNC-0.0.2
ENV RANCHER_HOME /var/lib/rancher

COPY gen_dnsmasq_conf $RANCHER_HOME/
COPY self.pem $RANCHER_HOME/
COPY inittab /etc/inittab

ADD $VNC.tar.gz $RANCHER_HOME/
COPY css/rancherVM.css $RANCHER_HOME/$VNC/css/
COPY ranchervm_logo.png $RANCHER_HOME/$VNC/images/
COPY index.html $RANCHER_HOME/$VNC/
COPY ranchervm $RANCHER_HOME/$VNC/cgi-bin/

ENTRYPOINT ["/bin/busybox", "init"]
VOLUME /vmmgmt
CMD []
