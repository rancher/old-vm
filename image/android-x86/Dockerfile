FROM rancher/vm-base:0.0.3
COPY android-4.4-r2.gz.qcow2 /base_image/
CMD ["-m 256m"]
ENV KVM_NET_OPTS="-netdev bridge,br=\$BRIDGE_IFACE,id=hostnet0 -device rtl8139,netdev=hostnet0,id=net0,mac=\$MAC,bus=pci.0,addr=0x3"
ENV KVM_BLK_OPTS="-hda \$KVM_IMAGE"
