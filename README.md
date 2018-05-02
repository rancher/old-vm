# RancherVM

[Package and run KVM images as Kubernetes pods, run at scale.](https://rancher.com/blog/2018/2018-04-27-ranchervm-now-available-on-kubernetes/)

## How It Works

RancherVM allows you to create VMs that run inside of Kubernetes pods, called
**VM Pods**. A VM pod looks and feels like a regular pod. Inside of each VM
pod, however, is a container running a virtual machine instance. You can
package any QEMU/KVM image as a Docker image, distribute it using any Docker
registry such as DockerHub, and run it on RancherVM.

RancherVM extends the Kubernetes API with [Custom Resource Definitions](https://kubernetes.io/docs/concepts/api-extension/custom-resources/), or CRDs.
Users define a VirtualMachine CRD specification detailing what base image, how
much compute resources and what keypairs are authorized to open an SSH session. 
A Kubernetes controller creates VM pods as necessary to achieve the desired
specification and reflects this in the VirtualMachine CRD status.

RancherVM comes with a Web UI for managing public keys, compute nodes, virtual
machines and accessing the VNC console from a web browser.

![How it works](docs/highlevel.svg "How it works")

## Run

Create a Kubernetes 1.8+ cluster and ensure KVM is installed on all nodes.
Follow the distribution-specific instructions to ensure KVM works. We only
require KVM to be enabled in the kernel. We do not need any user space tools
like `qemu-kvm` or `libvirt`. On Ubuntu 14.04, you can make sure KVM is enabled
by checking that both devices `/dev/kvm` and `/dev/net/tun` exist.

An easy way to run KVM on your Windows or Mac laptop is to use nested
virtualization with VMware Workstation or VMware Fusion. Just enable
"Virtualize Intel VT-x/EPT or AMD-V/RVI" in VM settings.

Once you have Kubernetes and KVM both setup, deploy the system:

```
kubectl create -f https://raw.githubusercontent.com/rancher/vm/master/hack/deploy.yaml
```

To determine the Web UI address, query for the frontend service:

```
kubectl get svc/ranchervm-frontend --namespace=ranchervm-system
```

This will return information on the frontend NodePort service similar to this:

```
NAME                 TYPE       CLUSTER-IP      EXTERNAL-IP   PORT(S)          AGE
ranchervm-frontend   NodePort   10.99.175.219   <none>        8000:32504/TCP   5h
```

Point your browser to `http://<node_ip>:32504`, replacing node_ip with the IP
address of any node in the cluster and `32504` with the external port you found
in the previous step.

You can create VM Pods through the Web UI or by creating Credential and
VirtualMachine CRDs:

```
kubectl create -f https://raw.githubusercontent.com/rancher/vm/master/hack/example/credentials.yaml
kubectl create -f https://raw.githubusercontent.com/rancher/vm/master/hack/example/virtualmachine.yaml
```

RancherVM is comprised of two Kubernetes controllers and a Web UI. Users may
manage VM Pods using the UI, by making API calls to the REST server backend, or
by directly creating/modifying CRDs.

## Build VM Images

You can find instructions on how to build images, including Windows images,
in the [RancherVM Images](docs/images.md) document.

## Networking

The details of how RancherVM configures network for the VM Pod is documented
in [RancherVM Networking](docs/networking.md).

## Build from Source

Just type `make`

RancherVM uses a modified version of noVNC at [`https://github.com/rancher/noVNC`](https://github.com/rancher/noVNC).
