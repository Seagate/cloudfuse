

ver=`../cloudfuse --version | cut -d " " -f 3`
image="azure-cloudfuse.$ver"

docker login blobfuse2containers.azurecr.io --username $1 --password $2
docker tag $image:latest blobfuse2containers.azurecr.io/$image
docker push blobfuse2containers.azurecr.io/$image
docker logout blobfuse2containers.azurecr.io
