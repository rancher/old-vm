FROM ubuntu:16.04

RUN \
  apt-get update -y && \
  DEBIAN_FRONTEND=noninteractive apt-get install -y git net-tools python python-numpy && \
  git clone https://github.com/novnc/noVNC /root/noVNC && \
  ln -s /root/noVNC/vnc.html /root/noVNC/index.html && \
  ln -s /root/noVNC/utils/launch.sh /usr/bin/novnc && \
  git clone https://github.com/novnc/websockify /root/noVNC/utils/websockify && \
  apt-get autoclean && \
  apt-get autoremove && \
  rm -rf /var/lib/apt/lists/*

# Overwrite the bundled script with one that supports unix socket target
COPY launch.sh /root/noVNC/utils/

