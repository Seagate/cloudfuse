
# Build cloudfuse binary
cd ..
echo "Building cloudfuse with libfuse"
./build.sh
ls -l cloudfuse

# As docker build can not go out of scope of this directory copy the binary here
cd -
cp ../cloudfuse ./
cp ../setup/11-cloudfuse.conf ./
cp ../setup/cloudfuse-logrotate ./

ver=`./cloudfuse --version | cut -d " " -f 3`
tag="cloudfuse.$ver"

# Cleanup older container image from docker
sudo docker image rm $tag -f

# Build new container image using current code
echo "Build container for libfuse3"
sudo docker build -t $tag -f $1 .

# List all images to verify if new image is created
sudo docker images

# Image build is executed so we can clean up temp executable from here
rm -rf ./cloudfuse
rm -rf 11-cloudfuse.conf cloudfuse-logrotate

# If build was successful then launch a container instance
status=`sudo docker images | grep $tag`
echo $status
