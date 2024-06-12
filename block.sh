#!/bin/bash

sudo iptables -A OUTPUT -d 192.55.6.0/24 -j REJECT