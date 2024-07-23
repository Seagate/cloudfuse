#!/bin/bash
#command to block outgoing calls to the cloud
sudo iptables -A OUTPUT -d 192.55.6.0/24 -j REJECT