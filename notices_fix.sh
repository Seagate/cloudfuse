#!/bin/bash

#==============================================================================
#title           : notices_fix.sh
#description     : Using go.sum file generates third-party notices for the project
#author		     : Vikas Bhansali (vibhansa@microsoft.com)
#date            : 02-Oct-2021
#usage		     : sh notices_fix.sh
#==============================================================================


# File to hold dependency list from go.sum file
dependency_list="./dependencies.lst"
output_file="./NOTICE"

# Function to create header for notices file
function dump_header
{
    # Dump static header to NOTICE file
    echo "--------------------- START OF THIRD PARTY NOTICE -----------------------------

    NOTICES AND INFORMATION
    Do Not Translate or Localize

    This software incorporates material from third parties.

    Notwithstanding any other terms, you may reverse engineer this software to the
    extent required to debug changes to any libraries licensed under the GNU Lesser
    General Public License.

    Third Party Programs: The software may include third party programs that Seagate,
    not the third party, licenses to you under this agreement. Notices, if any, for the
    third party programs are included for your information only.
    ---------------------------------------------------------------------------------------
    " > $output_file
}

function dump_footer
{
    echo -ne "--------------------- END OF THIRD PARTY NOTICE --------------------------------\n" >> $output_file
}

function append_lic_to_notice
{
    {
        echo
        echo -e "\n\n"
        echo "****************************************************************************"
        echo -e "\n============================================================================"
        echo -e ">>>" "$1"
        echo -e "=============================================================================="
        echo
        cat lic.tmp
    } >> $output_file

    rm -rf lic.tmp 
}

# Function to download the file and add it to Notice file with formatting
function download_and_dump
{
    #echo "Downloading lic for $1 from $2"
    # Cleanup old tmp file
    rm -rf lic.tmp

    if wget -q -O lic.tmp "$2"
    then
        append_lic_to_notice "$1"
        return 0
    fi

    return 1
}

function try_differ_names()
{
    # Try lic file without any extension
    if ! download_and_dump "$1" "$2"
    then
        # Try with .txt extension
        if ! download_and_dump "$line" "$lic_path".txt
            then
            # Try with .md extension
            download_and_dump "$line" "$lic_path".md
        fi
    fi

    return $?
}

function download_notice
{
    line=$1

    case $line in
    *go-autorest/*)
        # There are multiple entries with this for each of its subfolder so download only once
        if [[ $autorest_done -eq 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/Azure/go-autorest/master/LICENSE"

            if ! download_and_dump "github.com/Azure/go-autorest/autorest" $lic_path
            then
                # This might need manual intervention
                echo "Failed to get LICENSE from : AutoRest"
            else 
                autorest_done=1
            fi
        fi
        echo -ne "." ;;

    github.com*)
        # Try standard lic path first to get the info with 'master' branch
        # Define an array of possible license paths
        license_paths=(
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" --complement -f1)/master/LICENSE"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" -f2-3)/master/LICENSE"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" --complement -f1)/main/LICENSE"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" -f2-3)/main/LICENSE"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" --complement -f1)/master/COPYING"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" -f2-3)/master/License"
            "https://raw.githubusercontent.com/$(echo "$line" | cut -d "/" -f2-3)/main/License"
        )

        # Iterate over each license path and try to find the license
        license_found=false
        for lic_path in "${license_paths[@]}"; do
            if try_differ_names "$line" "$lic_path"; then
                license_found=true
                break
            fi
        done

        # If no license was found, print a failure message
        if ! $license_found; then
            echo "Failed to get LICENSE from: $line"
        fi

        echo -ne "." ;;

    *etcd.io/etcd*)
        # There are multiple entries with this for each of its subfolder so download only once
        if [[ $etcd_done -eq 0 ]]
        then
            lic_path="https://raw.githubusercontent.com/etcd-io/etcd/main/LICENSE"

            if ! download_and_dump "go.etcd.io/etcd" $lic_path
            then
                # This might need manual intervention
                echo "Failed to get LICENSE from : etcd.id"
            else 
                etcd_done=1
            fi
        fi
        echo -ne "." ;;


    *golang.org/x* | *rsc.io/* | *cloud.google.com/* | *google.golang.org/* | *go.uber.org/* | *go.opencensus.io* | *go.opentelemetry.io/*) 
        #echo ">>> " $line
        # Get the contents of this package
        if ! wget -q -O lic.tmp "https://pkg.go.dev/$line?tab=licenses"
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            # This will be html output so filter only license content
            sed -n '/License-contents/,/\/pre\>/p' lic.tmp > lic1.tmp
            head -1 lic1.tmp | grep "Copyright" | cut -d ">" -f 2 > lic.tmp
            sed '1d;$d' lic1.tmp >> lic.tmp

            # now dump it to our notice file
            append_lic_to_notice "$line"
        fi
        echo -ne "." ;;

    *gopkg.in/ini.v1*)
        if ! wget -q -O lic.tmp "https://raw.githubusercontent.com/go-ini/ini/v1.63.2/LICENSE"
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice "$line"
        fi

        echo -ne "." ;;

    *gopkg.in/*)
        #https://raw.githubusercontent.com/go-yaml/yaml/v3/LICENSE
        pkg=$(echo "$line" | cut -d "/" -f 2 | cut -d "." -f 1)
        ver=$(echo "$line" | cut -d "/" -f 2 | cut -d "." -f 2)
        if ! wget -q -O lic.tmp "https://raw.githubusercontent.com/go-$pkg/$pkg/$ver/LICENSE"
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice "$line"
        fi

        echo -ne "." ;;

    *dmitri.shuralyov.com*)
        #dmitri.shuralyov.com/gpu/mtl    
        # Get the contents of this package
        if ! wget -q -O lic.tmp "https://$line\$file/LICENSE"
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice "$line"
        fi
        echo -ne "." ;;

    *honnef.co/go/tools*)
        # Get the contents of this package
        if ! wget -q -O lic.tmp "https://raw.githubusercontent.com/dominikh/go-tools/master/LICENSE"
        then
            # This might need manual intervention
            echo "Failed to get LICENSE from : $line"
        else
            append_lic_to_notice "$line"
        fi
        echo -ne "." ;;

    *)
        echo "Others: " "$line";;
    esac
}

function generate_notices
{
    ret=0
    while IFS= read -r line; do
        case $line in
        *go-autorest/*)
            if grep -q ">>> github.com/Azure/go-autorest/autorest" $output_file
            then
                echo -ne "." 
            else
               #echo "Missing $line in old file" 
               download_notice "$line"
               ret=1
            fi
            echo -ne "." ;;

        *etcd.io/etcd*)
            if grep -q ">>> go.etcd.io/etcd" $output_file
            then
                echo -ne "." 
            else
               #echo "Missing $line in old file" 
               download_notice "$line"
               ret=1
            fi
            echo -ne "." ;;
            
        *)
            if grep -q ">>> $line" $output_file
            then
                echo -ne "."
            else
                #echo "Missing $line in old file"
                download_notice "$line"
                ret=1
            fi
            echo -ne "." ;;
        esac
    done < $dependency_list

    return $ret
}

function generate_qt_notice 
{
    # Add qt6 license only if it is not already in the notice file
    if grep -q "qt6" $output_file
    then
        echo -ne "." 
    else
        {  
            echo -e "\n\n"
            echo "****************************************************************************"
            echo -e "\n============================================================================"
            echo -e ">>> qt6"
            echo -e "=============================================================================="
            echo -e "                   GNU LESSER GENERAL PUBLIC LICENSE
                       Version 3, 29 June 2007

 Copyright (C) 2007 Free Software Foundation, Inc. <http://fsf.org/>
 Everyone is permitted to copy and distribute verbatim copies
 of this license document, but changing it is not allowed.

  This version of the GNU Lesser General Public License incorporates
the terms and conditions of version 3 of the GNU General Public
License, supplemented by the additional permissions listed below.

  0. Additional Definitions.

  As used herein, \"this License\" refers to version 3 of the GNU Lesser
General Public License, and the \"GNU GPL\" refers to version 3 of the GNU
General Public License.

  \"The Library\" refers to a covered work governed by this License,
other than an Application or a Combined Work as defined below.

  An \"Application\" is any work that makes use of an interface provided
by the Library, but which is not otherwise based on the Library.
Defining a subclass of a class defined by the Library is deemed a mode
of using an interface provided by the Library.

  A \"Combined Work\" is a work produced by combining or linking an
Application with the Library.  The particular version of the Library
with which the Combined Work was made is also called the \"Linked
Version\".

  The \"Minimal Corresponding Source\" for a Combined Work means the
Corresponding Source for the Combined Work, excluding any source code
for portions of the Combined Work that, considered in isolation, are
based on the Application, and not on the Linked Version.

  The \"Corresponding Application Code\" for a Combined Work means the
object code and/or source code for the Application, including any data
and utility programs needed for reproducing the Combined Work from the
Application, but excluding the System Libraries of the Combined Work.

  1. Exception to Section 3 of the GNU GPL.

  You may convey a covered work under sections 3 and 4 of this License
without being bound by section 3 of the GNU GPL.

  2. Conveying Modified Versions.

  If you modify a copy of the Library, and, in your modifications, a
facility refers to a function or data to be supplied by an Application
that uses the facility (other than as an argument passed when the
facility is invoked), then you may convey a copy of the modified
version:

   a) under this License, provided that you make a good faith effort to
   ensure that, in the event an Application does not supply the
   function or data, the facility still operates, and performs
   whatever part of its purpose remains meaningful, or

   b) under the GNU GPL, with none of the additional permissions of
   this License applicable to that copy.

  3. Object Code Incorporating Material from Library Header Files.

  The object code form of an Application may incorporate material from
a header file that is part of the Library.  You may convey such object
code under terms of your choice, provided that, if the incorporated
material is not limited to numerical parameters, data structure
layouts and accessors, or small macros, inline functions and templates
(ten or fewer lines in length), you do both of the following:

   a) Give prominent notice with each copy of the object code that the
   Library is used in it and that the Library and its use are
   covered by this License.

   b) Accompany the object code with a copy of the GNU GPL and this license
   document.

  4. Combined Works.

  You may convey a Combined Work under terms of your choice that,
taken together, effectively do not restrict modification of the
portions of the Library contained in the Combined Work and reverse
engineering for debugging such modifications, if you also do each of
the following:

   a) Give prominent notice with each copy of the Combined Work that
   the Library is used in it and that the Library and its use are
   covered by this License.

   b) Accompany the Combined Work with a copy of the GNU GPL and this license
   document.

   c) For a Combined Work that displays copyright notices during
   execution, include the copyright notice for the Library among
   these notices, as well as a reference directing the user to the
   copies of the GNU GPL and this license document.

   d) Do one of the following:

       0) Convey the Minimal Corresponding Source under the terms of this
       License, and the Corresponding Application Code in a form
       suitable for, and under terms that permit, the user to
       recombine or relink the Application with a modified version of
       the Linked Version to produce a modified Combined Work, in the
       manner specified by section 6 of the GNU GPL for conveying
       Corresponding Source.

       1) Use a suitable shared library mechanism for linking with the
       Library.  A suitable mechanism is one that (a) uses at run time
       a copy of the Library already present on the user's computer
       system, and (b) will operate properly with a modified version
       of the Library that is interface-compatible with the Linked
       Version.

   e) Provide Installation Information, but only if you would otherwise
   be required to provide such information under section 6 of the
   GNU GPL, and only to the extent that such information is
   necessary to install and execute a modified version of the
   Combined Work produced by recombining or relinking the
   Application with a modified version of the Linked Version. (If
   you use option 4d0, the Installation Information must accompany
   the Minimal Corresponding Source and Corresponding Application
   Code. If you use option 4d1, you must provide the Installation
   Information in the manner specified by section 6 of the GNU GPL
   for conveying Corresponding Source.)

  5. Combined Libraries.

  You may place library facilities that are a work based on the
Library side by side in a single library together with other library
facilities that are not Applications and are not covered by this
License, and convey such a combined library under terms of your
choice, if you do both of the following:

   a) Accompany the combined library with a copy of the same work based
   on the Library, uncombined with any other library facilities,
   conveyed under the terms of this License.

   b) Give prominent notice with the combined library that part of it
   is a work based on the Library, and explaining where to find the
   accompanying uncombined form of the same work.

  6. Revised Versions of the GNU Lesser General Public License.

  The Free Software Foundation may publish revised and/or new versions
of the GNU Lesser General Public License from time to time. Such new
versions will be similar in spirit to the present version, but may
differ in detail to address new problems or concerns.

  Each version is given a distinguishing version number. If the
Library as you received it specifies that a certain numbered version
of the GNU Lesser General Public License \"or any later version\"
applies to it, you have the option of following the terms and
conditions either of that published version or of any later version
published by the Free Software Foundation. If the Library as you
received it does not specify a version number of the GNU Lesser
General Public License, you may choose any version of the GNU Lesser
General Public License ever published by the Free Software Foundation.

  If the Library as you received it specifies that a proxy can decide
whether future versions of the GNU Lesser General Public License shall
apply, that proxy's public statement of acceptance of any version is
permanent authorization for you to choose that version for the
Library."
            echo
        }  >> $output_file
    fi
}


# Create temp directory for working on this 
rm -rf ./notice_tmp
mkdir ./notice_tmp/
chmod 777 ./notice_tmp/
cd ./notice_tmp/ || exit


# From go.sum file create unique list of dependencies we have
echo "Searching for dependencies"
< ../go.sum cut -d " " -f 1 | sort -u > $dependency_list

echo "github.com/winfsp/winfsp" >> $dependency_list

echo "Populating Notices"
# Check if notice.txt file exists or not
if [ -e ../$output_file ]
then
    # file is already there so copy that to temp folder
    # Ignore the file footer while making this copy
    echo "File exists check for new dependencies only"
    head -n -1 ../$output_file > $output_file
else
    # Main code to call the respective methods
    echo "File does not exists, start from scratch"
    dump_header
fi

# Generate notices in a temp file now
generate_notices

if ! generate_qt_notice
then
    # File is modified make space for fotter
    echo -e "\n" >> $output_file
fi
# Generate footer for the file
dump_footer

echo "Comparing missing dependencies"
# Compare the input list and notice file for final consolidation
grep ">>>" $output_file | cut -d " " -f 2 > notice.lst
diff $dependency_list notice.lst | grep -v "go-autorest" | grep -v "go.etcd.io"

# Delete the temp directory
cp $output_file ../NOTICE
cd - || exit
rm -rf ./notice_tmp/

echo "NOTICE updated..."


