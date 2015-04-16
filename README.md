# RancherVM

Package and run KVM images as Docker containers

## Run

First, ensure Docker and KVM are both installed on your system. Follow the
distribution-specific instructions to ensure KVM works. We only require
`qemu-kvm`. We do not need `libvirt`. On Ubuntu 14.04, you
can type `kvm-ok` to make sure KVM is supported.

    $ kvm-ok
    INFO: /dev/kvm exists
    KVM acceleration can be used

An easy way to run KVM on your Windows or Mac laptop is to use nested
virtualization with VMware Workstation or VMware Fusion. Just enable
"Virtualize Intel VT-x/EPT or AMD-V/RVI" in VM settings.

Once you have Docker and KVM both setup, run:

    docker run -v /var/run/docker.sock:/var/run/docker.sock \
        -p 8080:80 -v /tmp/ranchervm:/ranchervm rancher/ranchervm:0.0.1

and point your browser to `https://<KVM hostname>:8080`

You can create VM containers through the web UI or create them directly
using Docker command line as follows:

    docker run -e "RANCHER_VM=true" --cap-add NET_ADMIN -v \
        /tmp/ranchervm:/ranchervm --device /dev/kvm:/dev/kvm \
        --device /dev/net/tun:/dev/net/tun rancher/vm-rancheros

When you run a VM container from the command line, the system prints a
path to a Unix socket for VNC console access.

## Build VM Images

You can find instructions on how to build images, including Windows 
images, in the [RancherVM Images](docs/images.md) document.

## Networking

The details of how RancherVM configures network for the VM container
is documented in [RancherVM Networking](docs/networking.md).

## Build
Just type `make`

