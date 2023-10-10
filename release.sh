#!/bin/bash -x

# this is a bash script version of the packaging steps in cloudfuse-release.yaml

# checkout code (not necessary for local build)

# list last 12 hours of commits
git fetch --all --prune
git checkout main
git pull
git --no-pager log --since="12 hours ago" --stat

# install dependencies
sudo apt update --fix-missing
sudo apt-get install ruby-dev build-essential pkg-config cmake gcc g++ rpm libfuse-dev -y
sudo gem install fpm -V

# print GLIBC version
ldd --version

# start build-release.yml template

# Install Go
./go_installer.sh ~/Downloads/

# install Cloudfuse Go Dependencies
go get -d

# Build Cloudfuse and health monitor
./build.sh
# Verify Cloudfuse build
sudo chmod +x ./cloudfuse
./cloudfuse --version
# Verify health monitor build
sudo chmod +x ./cfusemon
./cfusemon --version

# end build-release.yml template

# stage files for packaging
cd ~
mkdir -p pkgDir/usr/bin/
mkdir -p pkgDir/usr/share/cloudfuse/
cp cloudfuse/cloudfuse pkgDir/usr/bin/cloudfuse
cp cloudfuse/cfusemon pkgDir/usr/bin/cfusemon
cp cloudfuse/setup/baseConfig.yaml pkgDir/usr/share/cloudfuse/
cp cloudfuse/sampleFileCacheConfig.yaml pkgDir/usr/share/cloudfuse/
cp cloudfuse/sampleStreamingConfig.yaml pkgDir/usr/share/cloudfuse/
mkdir -p pkgDir/etc/rsyslog.d
mkdir -p pkgDir/etc/logrotate.d
cp cloudfuse/setup/11-cloudfuse.conf pkgDir/etc/rsyslog.d
cp cloudfuse/setup/cloudfuse-logrotate pkgDir/etc/logrotate.d/cloudfuse

# using fpm tool for packaging of our binary & performing post-install operations
# for additional information about fpm refer https://fpm.readthedocs.io/en/v1.13.1/
BuildArtifactStagingDirectory=~/cfBASD
mkdir -p $BuildArtifactStagingDirectory
versionNumber=$(./pkgDir/usr/bin/cloudfuse --version | cut -d " " -f 3)
# make deb package
fpm -s dir -t deb -n cloudfuse -C pkgDir/ -v $versionNumber -d fuse \
    --maintainer "Seagate Cloudfuse Team" --url "https://github.com/Seagate/cloudfuse" \
    --description "A user-space filesystem for interacting with cloud storage" 
mv ./cloudfuse*.deb ./cloudfuse-$versionNumber.arm64.deb
cp ./cloudfuse*.deb $BuildArtifactStagingDirectory
# make rpm package
fpm -s dir -t rpm -n cloudfuse -C pkgDir/ -v $versionNumber -d $(depends) \
    --maintainer "Seagate Cloudfuse Team" --url "https://github.com/Seagate/cloudfuse" \
    --description "A user-space filesystem for interacting with cloud storage" 
mv ./cloudfuse*.rpm ./cloudfuse-$versionNumber.x86_64.rpm
cp ./cloudfuse*.rpm $BuildArtifactStagingDirectory

# list artifacts
sudo ls -lRt $BuildArtifactStagingDirectory
md5sum $BuildArtifactStagingDirectory/*

# All done! Time to publish the build products!

# Test artifacts
cd $BuildArtifactStagingDirectory
sudo dpkg --info cloudfuse*.deb
# sudo dpkg -i cloudfuse*.deb
# sudo apt install libfuse-dev build-essential -y

# start release-distro-tests.yml template

# end release-distro-tests.yml template