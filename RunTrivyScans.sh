#!/bin/bash

trivy fs --dependency-tree ./ > TrivyDependencyTree

trivy fs --scanners license ./ > TrivyLicenses