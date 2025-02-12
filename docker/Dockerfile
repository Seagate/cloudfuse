# Create container based on Ubuntu-22.04 Jammy Jellyfish image
FROM mcr.microsoft.com/mirror/docker/library/ubuntu:22.04

# Create directory to hold samples
RUN mkdir -p /usr/share/cloudfuse

# Copy cloudfuse binary to executable path
COPY ./cloudfuse /usr/local/bin/
COPY ./config.yaml /usr/share/cloudfuse/

# Install fuse library
RUN \
	apt update && \
	apt-get install -y ca-certificates vim rsyslog && \
	apt-get install -y fuse3

RUN echo "user_allow_other" >> /etc/fuse.conf

# Create syslog filter files
COPY ./11-cloudfuse.conf /etc/rsyslog.d
COPY ./cloudfuse-logrotate /etc/logrotate.d/cloudfuse


# Create mount directory structure
RUN \
	mkdir -p /mnt/blobfuse_mnt && \
	mkdir -p /tmp/blobfuse_temp && \
	chmod 777 /mnt/blobfuse_mnt && \
	chmod 777 /tmp/blobfuse_temp


# Create the mount script and set it to entry point once container start
RUN \
	echo "/sbin/rsyslogd" > /usr/share/cloudfuse/cloudfuse-mount.sh && \
	echo "cloudfuse mount /mnt/blobfuse_mnt --config-file=/usr/share/cloudfuse/config.yaml  --ignore-open-flags --foreground=true" >> /usr/share/cloudfuse/cloudfuse-mount.sh && \
	echo "cloudfuse unmount all" > /usr/share/cloudfuse/cloudfuse-umount.sh && \
	chmod 777 /usr/share/cloudfuse/cloudfuse-mount.sh && \
	chmod 777 /usr/share/cloudfuse/cloudfuse-umount.sh && \
	ln -s  /usr/share/cloudfuse/cloudfuse-mount.sh /usr/local/bin/fuse && \
	ln -s  /usr/share/cloudfuse/cloudfuse-umount.sh /usr/local/bin/unfuse


ENTRYPOINT ["bash", "fuse"]
