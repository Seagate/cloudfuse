# Create container based on Ubuntu-22.04 Jammy Jellyfish image
FROM ubuntu:latest

#Define arg to determine whether to build for fuse2 or fuse3
ARG FUSE2

# Create directory to hold samples
RUN mkdir -p /usr/share/lyvecloudfuse

# Copy lyvecloudfuse binary to executable path
COPY ./lyvecloudfuse /usr/local/bin/
COPY ./config.yaml /usr/share/lyvecloudfuse/

# Install fuse library
RUN \
	apt update && \
	apt-get install -y ca-certificates vim rsyslog

RUN if [ "$FUSE2" = "TRUE" ] ; then apt-get install -y fuse ; else apt-get install -y fuse3 ; fi 

RUN echo "user_allow_other" >> /etc/fuse.conf

# Create syslog filter files
COPY ./11-lyvecloudfuse.conf /etc/rsyslog.d
COPY ./lyvecloudfuse-logrotate /etc/logrotate.d/lyvecloudfuse


# Create mount directory structure
RUN \
	mkdir -p /mnt/blobfuse_mnt && \
	mkdir -p /tmp/blobfuse_temp && \
	chmod 777 /mnt/blobfuse_mnt && \
	chmod 777 /tmp/blobfuse_temp


# Create the mount script and set it to entry point once container start
RUN \
	echo "/sbin/rsyslogd" > /usr/share/lyvecloudfuse/lyvecloudfuse-mount.sh && \
	echo "lyvecloudfuse mount /mnt/blobfuse_mnt --config-file=/usr/share/lyvecloudfuse/config.yaml  --ignore-open-flags --foreground=true" >> /usr/share/lyvecloudfuse/lyvecloudfuse-mount.sh && \
	echo "lyvecloudfuse unmount all" > /usr/share/lyvecloudfuse/lyvecloudfuse-umount.sh && \
	chmod 777 /usr/share/lyvecloudfuse/lyvecloudfuse-mount.sh && \
	chmod 777 /usr/share/lyvecloudfuse/lyvecloudfuse-umount.sh && \
	ln -s  /usr/share/lyvecloudfuse/lyvecloudfuse-mount.sh /usr/local/bin/fuse && \
	ln -s  /usr/share/lyvecloudfuse/lyvecloudfuse-umount.sh /usr/local/bin/unfuse 


ENTRYPOINT ["bash", "fuse"]



