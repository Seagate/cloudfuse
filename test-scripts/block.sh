#!/bin/bash
#command to block outgoing calls to Lyve Cloud and Azure Blob Storage
sudo iptables -I OUTPUT 1 -p tcp -d 127.0.0.1 --dport 4566 -j REJECT
sudo iptables -I OUTPUT 1 -d 192.55.0.0/16 -j REJECT
sudo iptables -I OUTPUT 1 -d 20.60.0.0/16 -j REJECT
