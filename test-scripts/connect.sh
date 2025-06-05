#!/bin/bash
#command to accept outgoing calls to Lyve Cloud
sudo iptables -D OUTPUT -d 192.55.0.0/16 -j REJECT