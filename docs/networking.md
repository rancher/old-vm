# RancherVM Networking

RancherVM enables VM containers to be managed exactly like regular Docker
containers. The user performs exactly the same set of actions whether he
manages a regaulr Ubuntu container or a VM container running an
Unbuntu VM inside.

In order to make native container management experience work for virtual machines,
RancherVM transfers the networking configuration originally possessed by the container
to the virtual machine that resides inside the container. As a result, all native
Docker networking capabilities, such as virtual networking, port forwarding, and
container linking work transparently for virtual machines running inside VM containers.

Every VM container has an entry point called `startvm`. When a VM container is started, 
the `startvm` script performs the following steps to setup 
networking for the VM container.

1. The script records the current container's networking configuration (e.g.,
   IP address, MAC address, netmask, gateway, DNS server, and host name.)
1. The script creates a Linux bridge (`br0`) and connects the container's NIC (`eth0`)
   to the bridge.
1. The script removes the original IP address from container's NIC and generates a new
   non-conflicting IP address and MAC address for the bridge.
1. The script creates a `dnsmasq.conf` file and starts a `dnsmasq` process to serve as the
   dedicated DHCP server for one virtual machine. This DHCP server will only respond to 
   the original MAC and IP address of the container.
1. The script then runs `exec` of the KVM process. From now on QEMU/KVM runs as PID 1 in the VM container.

This approach transparently moves networking configuration from the container to
the virtual machine. Even though the `dnsmasq` DHCP server plays a central role here, it can
be disabled in deployments where other systems will provide DHCP and IPAM services to
virtual machines. For example, when we deploy RancherVM in Rancher, we disable the dnsmasq function
in VM containers. Rancher manages its own DHCP service, creates virtual networking, and allocates
IP addresses at a global scale.

The DHCP service offered by `dnsmasq` can be disabled by setting the `NO_DHCP` environment variable
to `true` before starting the VM container.
