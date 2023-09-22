#!/bin/bash

currYear=`date +"%Y"`
searchStr="Copyright ©"
copyLine=`grep -h "$searchStr" LICENSE`

if [[ "$1" == "replace" ]]
then 
    for i in $(find -name \*.go); do
        result=$(grep "$searchStr" $i)
        if [ $? -ne 1 ]
        then
            echo "Replacing in $i"
            result=$(grep "[+:]build" $i)
            #TODO: handle multiple compiler directives correctly
            #TODO: cound LICENSE lines instead of hardcoding
            if [ $? -ne 1 ]
            then
                sed -i -e '5,32{R LICENSE' -e 'd}' $i
            else
                sed -i -e '2,31{R LICENSE' -e 'd}' $i
            fi
        fi
    done
else
    for i in $(find -name \*.go); do
        result=$(grep "$searchStr" $i)
        if [ $? -eq 1 ]
        then
            echo "Adding Copyright to $i"
            # capture compilation directives
            result=$(grep "[+:]build" $i)
            if [ $? -ne 1 ]
            then
                echo "$result"  > __temp__
                echo -n >> __temp__
                echo "/*" >> __temp__
                cat LICENSE >> __temp__
                echo -e "*/" >> __temp__
                skipLines=$(($(grep -c "[+:]build" $i)+1))
                tail -n+$skipLines $i >> __temp__
            else
                echo "/*" > __temp__
                cat LICENSE >> __temp__
                echo -e "*/\n" >> __temp__
                cat $i >> __temp__
            fi
            mv __temp__ $i
        else
            currYear_found=$(echo $result | grep $currYear)
            if [ $? -eq 1 ]
            then
                #TODO: handle multiple copyright lines properly
                echo "Updating Copyright in $i"
                sed -i "/$searchStr/c\\$copyLine" $i
            fi
        fi
    done
fi