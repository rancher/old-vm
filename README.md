# RancherVM

Package and run KVM images as Docker containers

## Build

Just type `make`

## Run

First, ensure Docker and KVM are both installed on your system. Follow the
distribution-specific instructions to ensure KVM works. We only require
`qemu-kvm` and not `libvirt`. On Ubuntu 14.04, you
can type `kvm-ok` to make sure KVM is supported.

    $ kvm-ok
    INFO: /dev/kvm exists
    KVM acceleration can be used

An easy way to run KVM on your Windows or Mac laptop is to use nested
virtualization with VMware Workstation or VMware Fusion. Just enable
"Virtualize Intel VT-x/EPT or AMD-V/RVI" in VM settings.

Once you have Docker and KVM both setup, run:

    docker run -v /var/run/docker.sock:/var/run/docker.sock \
        -p 8080:80 rancher/ranchervm:0.0.1

and point your browser to `https://<KVM hostname>:8080`
