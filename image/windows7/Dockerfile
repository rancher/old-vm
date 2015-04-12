FROM rancher/vmbase:0.0.1
COPY windows7-starter.qcow2 /base_image/windows7-starter.qcow2
ENV KVM_BLK_DEV="ahci,id=ahci0,bus=pci.0,addr=0x6 -device ide-hd,bus=ahci0.0,id=sata0-0-0" KVM_NET_DEV="e1000"
#Windows 7 does not install with emulated LSI SCSI device.
#ENV KVM_BLK_DEV="lsi,id=scsi0,bus=pci.0,addr=0x7 -device scsi-hd,bus=scsi0.0,scsi-id=0,id=scsi0-0-0" KVM_NET_DEV="e1000"
#IDE works but is the least efficient
#ENV KVM_BLK_DEV="ide-hd,bus=ide.0,unit=0,id=ide0-0-0" KVM_NET_DEV="e1000"
