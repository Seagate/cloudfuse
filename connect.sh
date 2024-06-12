#!/bin/bash

sudo iptables -D OUTPUT -d 192.55.6.0/24 -j REJECT