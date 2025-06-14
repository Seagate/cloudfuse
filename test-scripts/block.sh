#!/bin/bash
#command to block outgoing calls to the Lyve Cloud
sudo iptables -I OUTPUT 1 -d 192.55.0.0/16 -j REJECT