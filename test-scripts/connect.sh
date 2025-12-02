#!/bin/bash
#command to accept outgoing calls to Lyve Cloud and Azure Blob Storage
sudo iptables -D OUTPUT -d 192.55.0.0/16 -j REJECT
sudo iptables -D OUTPUT -d 20.60.0.0/16 -j REJECT
