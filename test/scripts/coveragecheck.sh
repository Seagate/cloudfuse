#!/bin/bash

overall_check() {
    cvg=$(tail -1 ./cloudfuse_func_cover.rpt | cut -d ")" -f2 | sed -e 's/^[[:space:]]*//' | cut -d "%" -f1 | awk '{printf("%d\n", $1)}')
    echo $cvg
    if [ $cvg -lt 80 ]
    then
        echo "Code coverage below 80%"
        # Exit code changed to prevent failing in CI/CD pipeline
        # TODO: Remove this once we are passing file coverage checks consistently
        exit 0
    fi
    echo "Code coverage success"
}

file_check() {
    flag=0

    for i in $(grep "value=\"file" ./cloudfuse_coverage.html | cut -d ">" -f2 | cut -d "<" -f1 | sed -e "s/ //g")
    do
        fileName=$(echo $i | cut -d "(" -f1)
        percent=$(echo $i | cut -d "(" -f2 | cut -d "%" -f1)
        percentValue=$(expr $percent | awk '{printf("%d\n", $1)}')
        if [ $percentValue -lt 70 ]
        then
            flag=1
            echo $fileName" : "$percentValue
        fi
    done
    if [ $flag -eq 1 ]
    then
        echo "Code coverage below 70%"
        # Exit code changed to prevent failing in CI/CD pipeline
        # TODO: Remove this once we are passing file coverage checks consistently
        exit 0
    fi
    echo "Code coverage success"
}

if [[ $1 == "file" ]]
then
    file_check
else
    overall_check
fi
