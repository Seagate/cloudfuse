
# Build lyvecloudfuse binary
cd ..
if [ "$1" == "fuse2" ]
then
	echo "Building lyvecloudfuse with libfuse"
	./build.sh fuse2
else
	echo "Building lyvecloudfuse with libfuse3"
	./build.sh
fi

# As docker build can not go out of scope of this directory copy the binary here
cd -
cp ../lyvecloudfuse ./
cp ../setup/11-lyvecloudfuse.conf ./
cp ../setup/lyvecloudfuse-logrotate ./

ver=`./lyvecloudfuse --version | cut -d " " -f 3`
tag="azure-lyvecloudfuse.$ver"

# Cleanup older container image from docker
docker image rm $tag -f

# Build new container image using current code
if [ "$1" == "fuse2" ]
then
	echo "Build container for libfuse"
	docker build -t $tag -f Dockerfile . --build-arg FUSE2=TRUE
else
	echo "Build container for libfuse3"
	docker build -t $tag -f Dockerfile .
fi
 
# Image build is executed so we can clean up temp executable from here
rm -rf ./lyvecloudfuse
rm -rf 11-lyvecloudfuse.conf lyvecloudfuse-logrotate

# If build was successful then launch a container instance
status=`docker images | grep $tag`
