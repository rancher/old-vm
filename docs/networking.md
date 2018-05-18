# RancherVM Networking

RancherVM enables VM containers to be managed exactly like regular Docker
containers. The user performs the same set of actions whether he
manages a regular Ubuntu container or a VM container running an
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
   dedicated DHCP server for the one virtual machine running inside the container. This 
   DHCP server will only respond to the original MAC and IP address of the container.
1. The script then runs `exec` of the KVM process. KVM is bridged to the newly created
   Linux bridge `br0`. From now on QEMU/KVM runs as PID 1 in the VM container.

This approach transparently moves networking configuration from the container to
the virtual machine. 

The DHCP service offered by `dnsmasq` can be disabled by setting the `NO_DHCP` environment variable
to `true` before starting the VM container.
The `dnsmasq` DHCP server should be disabled where another system will provide DHCP and IPAM services to
virtual machines. For example, when we deploy RancherVM in Rancher, we disable the dnsmasq function
in VM containers. Rancher manages its own DHCP service, creates virtual networking, and allocates
IP addresses from its own database. Other deployments may use alternative means of configuring
virtual machine networking. As an example, cloud-init also eliminates the need for DHCP.

## Bridge to the Host Network

It is possible to configure RancherVM to bridge its eth0 to host network.

Assuming you are running Ubuntu host and your host is connected to its physical network via `eth0`. If you run CentOS or Fedora, the step should be similar, but you need to use the corresponding networking commands for your Linux distro.

First you need to create a bridge in your host for `eth0`. Setup your `/etc/network/interfaces` file as follows:

    auto lo
    iface lo inet loopback

    auto eth0
    iface eth0 inet manual

    auto br0
    iface br0 inet dhcp
        bridge_ports eth0
        bridge_stp off
        bridge_fd 0
        bridge_maxwait 0

Running ifconfig on the host should give you something like this:

    # ifconfig
    br0       Link encap:Ethernet  HWaddr 00:0c:29:24:8a:81  
          inet addr:192.168.111.158  Bcast:192.168.111.255  Mask:255.255.255.0
          inet6 addr: fe80::20c:29ff:fe24:8a81/64 Scope:Link
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:2354 errors:0 dropped:0 overruns:0 frame:0
          TX packets:2012 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:0 
          RX bytes:1202270 (1.2 MB)  TX bytes:250767 (250.7 KB)

    eth0      Link encap:Ethernet  HWaddr 00:0c:29:24:8a:81  
          UP BROADCAST RUNNING MULTICAST  MTU:1500  Metric:1
          RX packets:1539 errors:0 dropped:0 overruns:0 frame:0
          TX packets:1347 errors:0 dropped:0 overruns:0 carrier:0
          collisions:0 txqueuelen:1000 
          RX bytes:580472 (580.4 KB)  TX bytes:209948 (209.9 KB)

Note IP address is now set on `br0`. And `eth0` and `br0` have the same MAC.

You will need to disable packet filtering for bridge traffic. At the time of
writing, there is no _correct_ way to persist bridge netfilter settings across
reboot. Merely setting them in `/etc/sysctl.conf` doesn't work due to a timing
issue. See [this document](https://wiki.libvirt.org/page/Net.bridge.bridge-nf-call_and_sysctl.conf) for more information and possible solutions.
If you just want a quick solution, dumping the following into `/etc/rc.local`
will probably work:

    modprobe br_netfilter
    echo 0 > /proc/sys/net/bridge/bridge-nf-call-arptables
    echo 0 > /proc/sys/net/bridge/bridge-nf-call-iptables
    echo 0 > /proc/sys/net/bridge/bridge-nf-call-ip6tables 
    sysctl -p
