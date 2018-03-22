#!/bin/bash

set -e

echo "Starting Rancher VM Base container"

echo "Copying binaries"

cp /var/lib/rancher/startvm /rancher/startvm

echo "Successfully copied binaries"

sleep infinity
