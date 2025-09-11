#!/bin/bash

# Build the Go application
cd /home/cjh/work/source/image-builder/go
go build -o imagebuilder cmd/imagebuilder/main.go

echo "Testing basic buildah functionality..."
# Test basic buildah functionality
sudo buildah from --name test-container fedora:latest
sudo buildah run test-container -- ls -la /
sudo buildah commit test-container test-basic-image
sudo buildah rm test-container
sudo buildah images | grep test-basic-image

# Instead of running our full application, let's run a simplified test
echo -e "\nTesting our Go imagebuilder..."
# Create a simple test image without package installation
sudo ./imagebuilder \
  -layer_type=base \
  -parent=fedora:latest \
  -name=test-fedora-image

# Check the images
sudo buildah images | grep test-fedora
