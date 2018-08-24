## Default credentials:
- Username: `ubuntu`
- Password: `ubuntu`

## How to build

```
curl -LO https://s3-us-west-1.amazonaws.com/ranchervm/iso/ubuntu-16.04.4-server-amd64.iso
qemu-img create -f qcow2 ubuntu-16.04.4-server-amd64.img 50G
qemu-system-x86_64 -enable-kvm -m size=4096 -smp cpus=1 -vnc 0.0.0.0:0 -cdrom ubuntu-16.04.4-server-amd64.iso -drive file=ubuntu-16.04.4-server-amd64.img -netdev bridge,br=br0,id=net0 -device virtio-net-pci,netdev=net0,mac=06:fe:a7:1d:03:c5
# Perform installation - guided w/LVM, add OpenSSH Server
# Reboot, then turn off password SSH, install python and cloud-init, disable swap, clear history
# Pull any Docker images that should be cached now
qemu-img convert -O qcow2 -c ubuntu-16.04.4-server-amd64.img ubuntu-16.04.4-server-amd64.qcow2
```

