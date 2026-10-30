#!/bin/bash

# The dependency list is computed with 'go list -deps' over every binary and
# platform built by .goreleaser.yaml, so only code that is actually shipped is
# listed. License and NOTICE files are read from the Go module cache at the
# exact version recorded in go.sum, so the output is reproducible and matches
# what is compiled in.
#
# If a license cannot be resolved, a warning is printed and the existing
# section from the current NOTICE file (if any) is kept.

export LC_ALL=C

output_file="./NOTICE"

# WinFsp is not a Go module, but its installer is bundled with the Windows
# installer, so its license is included as well. Keep in sync with
# build/windows_installer_build.iss.
winfsp_tag="v2.1"

# Targets built by .goreleaser.yaml as "GOOS GOARCH CGO_ENABLED TAGS PACKAGE"
build_targets=(
    "linux amd64 1 fuse3 ."
    "linux amd64 1 - ."
    "linux arm64 1 fuse3 ."
    "linux arm64 1 - ."
    "linux amd64 0 - ./tools/health-monitor"
    "linux arm64 0 - ./tools/health-monitor"
    "windows amd64 0 - ."
    "windows amd64 0 - ./tools/windows-startup"
    "windows amd64 0 - ./tools/windows-service"
    "windows amd64 0 - ./tools/health-monitor"
)

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

new_notice="$work_dir/NOTICE"
old_notice="$work_dir/NOTICE.old"
if [ -e $output_file ]; then
    tr -d '\r' < $output_file > "$old_notice"
else
    touch "$old_notice"
fi

warnings=0

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
    " > "$new_notice"
}

function dump_footer
{
    echo -ne "--------------------- END OF THIRD PARTY NOTICE --------------------------------\n" >> "$new_notice"
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
        tr -d '\r' < "$work_dir/lic.tmp"
    } >> "$new_notice"

    rm -f "$work_dir/lic.tmp"
}

# Copy the section for a dependency from the existing NOTICE file, used when its
# license can no longer be resolved
function keep_old_section
{
    # Section body runs from the "====" line after ">>> name" to the next "****" line
    awk -v name="$1" '
        /^>>> / { found = ($2 == name); in_header = found; next }
        in_header { if (/^=+$/) in_header = 0; next }
        /^\*{20,}$/ || /END OF THIRD PARTY NOTICE/ { found = 0 }
        found { print }
    ' "$old_notice" | sed -e '/./,$!d' | sed -e :a -e '/^\n*$/{$d;N;ba' -e '}' > "$work_dir/lic.tmp"

    if [ -s "$work_dir/lic.tmp" ]; then
        echo "WARNING: could not resolve license for $1, keeping existing section"
        append_lic_to_notice "$1"
    else
        echo "WARNING: could not resolve license for $1 and no existing section found"
        rm -f "$work_dir/lic.tmp"
    fi
    warnings=1
}

# Append the license and NOTICE files in a directory to lic.tmp, labelling each
# file with the given prefix when it is not the main module license
function collect_files
{
    local dir=$1
    local prefix=$2
    local pattern file

    # License files first, then any upstream NOTICE files (Apache-2.0 section 4(d))
    for pattern in '^(licen[cs]e|copying)' '^notice'; do
        while IFS= read -r file; do
            [ -s "$work_dir/lic.tmp" ] && printf '\n' >> "$work_dir/lic.tmp"
            if [[ $pattern == '^notice' ]] || [ -n "$prefix" ]; then
                printf -- '------------------------------ %s ------------------------------\n\n' "$prefix$file" >> "$work_dir/lic.tmp"
            fi
            cat "$dir/$file" >> "$work_dir/lic.tmp"
        done < <(find "$dir" -maxdepth 1 -type f -printf '%f\n' | grep -iE "$pattern" | sort)
    done
}

# Add the license and NOTICE files of a module, including those in compiled
# packages below the module root (e.g. vendored third-party code)
function dump_module
{
    local name=$1
    local dir=$2
    local pkg_dir

    rm -f "$work_dir/lic.tmp"
    if [ -n "$dir" ] && [ -d "$dir" ]; then
        collect_files "$dir" ""
        while read -r _ pkg_dir; do
            [ "$pkg_dir" != "$dir" ] && collect_files "$pkg_dir" "${pkg_dir#"$dir"/}/"
        done < <(awk -v name="$name" '$1 == name' "$work_dir/pkg_dirs.lst")
    fi

    if [ -s "$work_dir/lic.tmp" ]; then
        append_lic_to_notice "$name"
    else
        keep_old_section "$name"
    fi
}

echo "Searching for dependencies"
for target in "${build_targets[@]}"; do
    read -r goos goarch cgo tags pkg <<< "$target"
    [ "$tags" = "-" ] && tags=""
    if ! GOOS=$goos GOARCH=$goarch CGO_ENABLED=$cgo go list -deps -tags="$tags" \
        -f '{{with .Module}}{{.Path}} {{with .Replace}}{{.Dir}}{{else}}{{.Dir}}{{end}} {{$.Dir}}{{end}}' \
        "$pkg" >> "$work_dir/deps.lst"; then
        echo "WARNING: go list failed for $target"
        warnings=1
    fi
done

main_module=$(go list -m)
grep -v "^$main_module " "$work_dir/deps.lst" | sort -u > "$work_dir/deps_all.lst"
# "module module_dir" for each module, and "module package_dir" for each package
cut -d " " -f 1,2 "$work_dir/deps_all.lst" | sort -u > "$work_dir/deps_sorted.lst"
cut -d " " -f 1,3 "$work_dir/deps_all.lst" | sort -u > "$work_dir/pkg_dirs.lst"

echo "Populating Notices"
dump_header

while read -r name dir; do
    dump_module "$name" "$dir"
done < "$work_dir/deps_sorted.lst"

# The Go standard library and runtime are statically linked into every binary
dump_module "github.com/golang/go" "$(go env GOROOT)"

if curl -sfL -o "$work_dir/lic.tmp" "https://raw.githubusercontent.com/winfsp/winfsp/$winfsp_tag/License.txt"; then
    append_lic_to_notice "github.com/winfsp/winfsp"
else
    keep_old_section "github.com/winfsp/winfsp"
fi

dump_footer

cp "$new_notice" $output_file

if [ $warnings -ne 0 ]; then
    echo "NOTICE updated with warnings, review the messages above"
else
    echo "NOTICE updated..."
fi
