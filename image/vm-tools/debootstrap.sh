#!/bin/bash -ex

TARGET_ROOTFS_DIR=/tmp/rootfs/RancherVM-debootstrap-ubuntu-1804
mkdir -p ${TARGET_ROOTFS_DIR}

INCLUDE_PKGS="vim,qemu-kvm,qemu-utils,bridge-utils,genisoimage,curl,net-tools"

debootstrap \
  --arch=amd64 \
  --variant=minbase \
  --include="${INCLUDE_PKGS}" \
  bionic \
  ${TARGET_ROOTFS_DIR} \
  http://archive.ubuntu.com/ubuntu/

rm -rf ${TARGET_ROOTFS_DIR}/var/cache/apt/archives/*.deb
rm -rf ${TARGET_ROOTFS_DIR}/usr/share/man
rm -rf ${TARGET_ROOTFS_DIR}/usr/share/locale

tar cvzf ${TARGET_ROOTFS_DIR}.tgz -C ${TARGET_ROOTFS_DIR} .
