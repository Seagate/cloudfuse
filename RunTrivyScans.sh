#!/bin/bash

trivy fs ./ --scanners license --exit-code 1 --severity HIGH,CRITICAL

trivy fs ./ --exit-code 1 --severity MEDIUM,HIGH,CRITICAL --dependency-tree