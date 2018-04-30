This is an example of creating a VM container using emulated QEMU networking
and storage devices. The procedure applies to any operating systems that do 
not support virtio drivers.

Build the Docker image from an Android x86 image from 
http://www.android-x86.org/

1. Download android-x86-4.4-r2.iso from http://www.android-x86.org/

2. Create the blank qcow2 image:

$ qemu-img create -f qcow2 android.qcow2 4G

3. Start KVM and mount the disk image and CDROM ISO:

$ kvm -hda android.qcow2 -cdrom android-x86-4.4-r2.iso

4. Follow instructions at http://www.android-x86.org/documents/installhowto to 
select the "Installation" option, create a partition for the entire disk,
and install Android onto the partition.

5. Compress the qcow2 image for packaging:

$ qemu-img convert -O qcow2 -c android.qcow2 android.gz.qcow2

6. Copy the resulting android.gz.qcow2 file into the current directory and
rename it android-4.4-r2.gz.qcow2.

7. Run "make" to build the Docker image for Android-x86. Note that the Makefile
will automatically download a pre-built version of android-4.4-r2.gz.qcow2
if we had not created that file ourself and placed it in the current
directory.

The Android x86 VM works. But I have observed 2 problems:

1. It pegs the CPU at 100%
2. The console screen goes off after a brief period of inactivity, and I don't
know how to bring the screen back up
