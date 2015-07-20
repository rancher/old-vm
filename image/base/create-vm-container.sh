BASE_IMAGE_HOST=~/ranchervm-images

VM_NAME=$1
IMAGE=$2

[ -z "$1" ] && IMAGE=rancheros


die(){
	echo $1
	exit 1
}
if [ -z "$VM_NAME" ]; then
	die "Usage: $0 vm_name [image]"
fi


if [ ! -f "$BASE_IMAGE_HOST/$IMAGE" ]; then
	die "Error: no image dir $BASE_IMAGE_HOST/$IMAGE"
fi

NEW_ID=$(docker run -d -e RANCHER_VM=true \
	--cap-add NET_ADMIN \
	-e IMAGE=$IMAGE \
	-v $BASE_IMAGE_HOST:/base_image \
	-v /tmp/ranchervm:/ranchervm \
	--device /dev/kvm:/dev/kvm \
	--device /dev/net/tun:/dev/net/tun \
	--name $VM_NAME \
	rancher/vm-base)

echo "Created container $NEW_ID"
sleep 1
docker logs $NEW_ID

