FROM ubuntu:14.04
RUN apt-get update && apt-get install -y inotify-tools dnsmasq python-minimal
ADD https://bootstrap.pypa.io/get-pip.py /usr/bin/
RUN python /usr/bin/get-pip.py
RUN pip install docker-py
COPY ranchervm /var/lib/rancher/ranchervm
COPY gen_dnsmasq_conf /var/lib/rancher/gen_dnsmasq_conf
COPY startmgmt /var/lib/rancher/startmgmt
COPY self.pem /var/lib/rancher/self.pem
COPY noVNC-0.0.2 /var/lib/rancher/noVNC-0.0.2
ENTRYPOINT ["/var/lib/rancher/startmgmt"]
CMD []

