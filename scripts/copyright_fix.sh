#!/bin/bash

# Update LICENSE and component.template files with correct year (Eg: "Copyright © 2020-2025" to "Copyright © 2020-2026") and run the script ./copyright.sh
currYear=$(date +"%Y")
searchStr="Copyright ©"
copyLine=$(grep -h "$searchStr" LICENSE)

if [[ "$1" == "replace" ]]
then
    find . -name '*.go' -print0 | while IFS= read -r -d '' i; do
        result=$(grep "$searchStr" "$i")
        if [ $? -ne 1 ]
        then
            echo "Replacing in $i"
            # find the start and end line of the existing license
            firstLine=$(($(grep -n -m 1 "\/\*" "$i" | cut -f1 -d:)+1))
            lastLine=$(($(grep -n -m 1 "\*\/" "$i" | cut -f1 -d:)-1))
            sed -i -e "$firstLine,$lastLine{R LICENSE" -e "d}" "$i"
        fi
    done
else
    find . -name '*.go' -print0 | while IFS= read -r -d '' i; do
        result=$(grep "$searchStr" "$i")
        if [ $? -eq 1 ]; then
            echo "Adding Copyright to $i"
            # capture compilation directives
            result=$(grep "[+:]build" "$i")
            if [ $? -ne 1 ]; then
                echo "$result" > __temp__
                {
                    echo -n
                    echo "/*"
                    cat LICENSE
                    echo -e "*/"
                    skipLines=$(($(grep -c "[+:]build" "$i")+1))
                    tail -n+$skipLines "$i"
                } >> __temp__
            else
                echo "/*" > __temp__
                {
                    cat LICENSE
                    echo -e "*/\n"
                    cat "$i"
                } >> __temp__
            fi
            mv __temp__ "$i"
        else
            _=$(echo "$result" | grep "$currYear")
            if [ $? -eq 1 ]; then
                #TODO: handle multiple copyright lines properly
                echo "Updating Copyright in $i"
                sed -i "/$searchStr/c\\$copyLine" "$i"
            fi
        fi
    done
fi
