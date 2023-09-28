# Windows Support

Cloudfuse supports running on Windows either as an executable in foreground mode or as a Windows services. Cloudfuse
requires the third party utility [WinFsp](https://winfsp.dev/). To download WinFsp, please run the WinFsp installer
found [here](https://winfsp.dev/rel/).

## Running in foreground mode
To run in foreground mode simply start the mount using the `mount` command. All mounts on Windows started this way
automatically run in foreground.

        cloudfuse.exe mount <mount path> --config-file=<config file>

## Running as a Windows service (recommended)
To run as a Windows service you need to run the following commands in a terminal with administrator privileges.

1. Install Cloudfuse as a Windows service.

        cloudfuse.exe service install

2. Start the Cloudfuse Windows service.

        cloudfuse.exe service start

3. Now we can start a mount that is managed by Cloudfuse. Once you mount the bucket or container the mount will persit
   on restart or shutdowns while the Cloudfuse service is running. Cloudfuse can also support any number of mounts
   running on Windows.

        cloudfuse.exe service mount <mount path> --config-file=<config file>

To unmount a specific instance use the unmount command. This will also prevent this mount from persiting on restarts.

        cloudfuse.exe service unmount <mount path>

To stop the Cloudfuse service use the stop command. Mounts that were running will reappear once the service is started
again.

        cloudfuse.exe service stop

To uninstall Cloudfuse as a Windows service use the uninstall command.

        cloudfuse.exe service uninstall

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
| `|` | `｜` |
| `?` | `？` |
| `*` | `＊` |

This can lead to issues if you have files on Windows that include those Unicode characters as they will be converted on
upload. It is not possible to distinguish in this case which characters should be replaced, so if this is a usecase then
you should not enable the optional flag.
