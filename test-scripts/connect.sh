#!/bin/bash
#command to accept outgoing calls to the cloud
sudo iptables -D OUTPUT -d 192.55.6.0/24 -j REJECT
sudo iptables -D OUTPUT -d 20.60.153.33 -j REJECT