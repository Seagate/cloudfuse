# Windows Support

Cloudfuse supports running on Windows either as an executable in foreground mode or as a Windows services. Cloudfuse
requires the third party utility [WinFsp](https://winfsp.dev/). WinFSP installs automatically with Cloudfuse, but you
can also run the WinFsp installer found [here](https://winfsp.dev/rel/) yourself.

## Running in foreground mode

To run in foreground mode, you must pass the option `--foreground=true` when using the `mount` command.

        cloudfuse.exe mount <mount path> --config-file=<config file> --foreground=true

## Running in background mode (recommended)

Cloudfuse runs in the background by default. It uses the WinFSP launcher to run the mount in the background.

        cloudfuse.exe mount <mount path> --config-file=<config file>

Cloudfuse can also automatically restart existing mounts on user login. To do so, pass the --enable-remount flag
when mounting

        cloudfuse.exe mount <mount path> --config-file=<config file> --enable-remount

To unmount a specific instance, use the unmount command.

        cloudfuse.exe unmount <mount path>

To unmount and also prevent this mount from persisting on restarts pass the --disable-remount flag.

        cloudfuse.exe unmount <mount path> --disable-remount.

Cloudfuse supports mounting any number of buckets.

If the container is not automatically mounted on user login after a reboot, you may need to (re)install the Cloudfuse
startup program:

        cloudfuse.exe service install

To uninstall the Cloudfuse startup program use the uninstall command.

        cloudfuse.exe service uninstall

## Windows Security and User Permissions

By default, cloudfuse allows all users to read/write to the mounted directory. If you need specific permissions for your
share you must provide them in your config file when you mount using cloudfuse. This requires you to generate and use
Security Descriptor Definition Language (SDDL) strings to manage permissions. We provide a brief example for how to
generate SDDL strings to change permissions.

1. Ensure that you have WinFsp installed on your system:

   If you used the standard cloudfuse installer on Windows, then WinFsp is already installed on your system.

2. Find Your Account's SID and UID:

   Use the fsptool utility to discover your account's SID and UID. Open a command prompt and run:

        'C:\Program Files (x86)\WinFsp\bin\fsptool-x64.exe' id

   This will output something like:

        User=S-1-5-21-773277305-2169295204-1991566178-478888(user) (uid=21479625)
        Owner=S-1-5-21-773277305-2169295204-1991566178-478888(user) (uid=21479625)
        Group=S-1-5-21-773277305-2169295204-1991566178-478888(user) (gid=21479625)

3. Generate SDDL for Specific Permissions: Use the fsptool to generate the SDDL string for your account's UID with
specific permissions. For example, to generate an SDDL for rwx------ permissions:

        'C:\Program Files (x86)\WinFsp\bin\fsptool-x64.exe' perm 21479625:0:700

   This will output something like:

        O:S-1-0-65534G:S-1-5-0D:P(A;;0x1f01bf;;;S-1-0-65534)(A;;0x120088;;;S-1-5-0)(A;;0x120088;;;WD) (perm=65534:0:0700)

4. Edit your config file: Edit the libfuse section of the config.yaml file to include the windows-sddl entry. Add the
SDDL string you generated in the previous step. For example:

        libfuse:
          windows-sddl: O:S-1-0-65534G:S-1-5-0D:P(A;;0x1f01bf;;;S-1-0-65534)(A;;0x120088;;;S-1-5-0)(A;;0x120088;;;WD)

## Filename Limitations

As Cloudfuse supports both Windows and Linux as well as Azure and S3 storage, there are naming restrictions that must be
followed in order for data to be available on all systems.

Cloudfuse does not support filenames longer than 255 characters. It does not support filenames that contain the `\`
character or any control characters (ASCII 0 -31). On Windows it can support paths with a length longer than 255 if you
enable the LongPathsEnabled registry option. See
<https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation?tabs=powershell>.

If the restricted-characters-windows is enabled in S3 or Azure storage, then Windows will be able to display filenames
with the following restricted characters `<>:"|?*` that are valid on Linux. These characters are restricted on Windows,
but to allow them to be seen on Windows they will be replaced with similar looking Unicode characters.

| Character | Replacement |
| ---------- | ---------- |
| `<` | `＜` |
| `>` | `＞` |
| `:` | `：` |
| `"` | `＂` |
| `\|` | `｜` |
| `?` | `？` |
| `*` | `＊` |

This can lead to issues if you have files on Windows that include those Unicode characters as they will be converted on
upload. It is not possible to distinguish in this case which characters should be replaced, so if this is a usecase then
you should not enable the optional flag.
