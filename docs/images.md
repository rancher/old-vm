# RancherVM Images

RancherVM images are standard KVM images bundled in Docker images. We recommend
you use compressed `qcow2` images. Any format that can serve as a QEMU base image will also
work, but compressed `qcow2` images are preferred due to its small size.

## Build VM Images

There are many tools you can use to build VM images. One tool we particularly
like is `virt-manager`. It allows you to configure virtio devices
and install operating systems from bootable ISO files. 

On Ubuntu (and perhaps on other Linux distributions as well) `virt-manager` places the
resulting `qcow2` image in a directory called `/var/lib/libvirt/images/`. You
need to be root to read the `qcow2` images in that directory.

## Compress VM Images

After you have installed the operating system in a `qcow2` file and created
a file like `myos.img`, run the `qemu-img convert` command to compress the image.
For example:

    qemu-img convert -O qcow2 -c myos.img myos.gz.img

Image compression can lead to over 50% size reduction.

## Build VM Container Images

You can follow the example of one
of the VM images included with RancherVM to build your own VM image.
The following, for example, is the Dockerfile for building RancherOS
VM:

    FROM rancher/vm-base:0.0.1
    COPY rancheros-0.3.0-gz.img /base_image/rancheros-0.3.0-gz.img
    CMD ["-m 512m"]

All VM containers must use `rancher/vm-base` as the base image. You can give
the `qcow2` image any name you want. The `qcow2` image must be copied
into the `/base_image` directory. There must be exactly one image file
in that directory.

Note that the `-m 512` option is passed to KVM. The `CMD` commands or any
command line options following the Docker image argument in `docker run`
are passed verbatim to KVM.

## Virtio Drivers

Preferrably, the images are built 
to work with KVM virto drivers. It is possible to configure your VM
container to work with other storage and network drivers as well.
RancherVM uses the following KVM command line options to configure storage
and networking devices. You can redefine one or both of these environment
valuables (in Dockerfile or docker run command line) to the device of your
choice.

    KVM_BLK_OPTS="-drive file=\$KVM_IMAGE,if=none,id=drive-disk0,format=qcow2 -device virtio-blk-pci,scsi=off,bus=pci.0,addr=0x6,drive=drive-disk0,id=virtio-disk0,bootindex=1"
    KVM_NET_OPTS="-netdev bridge,br=\$BRIDGE_IFACE,id=net0 -device virtio-net-pci,netdev=net0,mac=\$MAC"

Note that RancherVM scripts will substitute in the right value for `$KVM_IMAGE`,
`$BRIDGE_IFACE`, and `$MAC`. 

1. Place `$KVM_IMAGE` in the block device option where the `qcow2` image file is specified.
1. Make sure KVM networking is configured as bridge mode and the virtual NIC is bridged to `$BRIDGE_IFACE`.
1. Make sure the MAC address of the virtual NIC is set to `$MAC`.

Do not mess with these valriables. Do not remove the `\` before `$` character. Place these variables exactly
where they should be. If you do not know where they should go, you have probably
messed up the command line options.

You can use `virt-manager` to figure out what command line options you need to use
should you decide to customize `KVM_BLK_OPTS` and `KVM_NET_OPTS`. Just configure
what you want in `virt-manager`, start the VM, and type `ps -ef | grep kvm` to see what command
line options `virt-manager` has generated.

## Build Windows VM Images

Linux distributions generally support virtio out of the box. You need to take
special steps to install virtio drivers for Windows. This [video](https://youtu.be/VAWKHrfDWrM) explains
how to build a Windows image for RancherVM.

