#!/bin/bash
#command to block outgoing calls to the cloud
sudo iptables -A OUTPUT -d 192.55.6.0/24 -j REJECT
sudo iptables -A OUTPUT -d 20.60.153.33 -j REJECT