#!/bin/bash -ex

if [ ! -d /vm-tools ]; then
  echo "error: /vm-tools not mounted, exiting"
  exit 1
fi

echo "Extracting to vm-tools"
tar xzf /opt/rancher/vm-tools/ubuntu_1604.tar.gz -C /vm-tools
cp /opt/rancher/vm-tools/startvm /vm-tools/usr/bin/startvm
echo "Extraction successful"

exit 0
