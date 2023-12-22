# Windows Support

Cloudfuse supports running on Windows either as an executable in foreground mode or as a Windows services. Cloudfuse
requires the third party utility [WinFsp](https://winfsp.dev/). WinFSP installs automatically with Cloudfuse, but you
can also run the WinFsp installer found [here](https://winfsp.dev/rel/) yourself.

## Running in foreground mode
To run in foreground mode, you must pass the option `--foreground=true` when using the `mount` command.

        cloudfuse.exe mount <mount path> --config-file=<config file> --foreground=true

## Running in background mode (recommended)
Cloudfuse runs in the background by default. It uses the WinFSP launcher to run the mount in the background.
Cloudfuse will also automatically restart existing mounts on user login.

        cloudfuse.exe mount <mount path> --config-file=<config file>

To unmount a specific instance, use the unmount command. This will also prevent this mount from persisting on restarts.

        cloudfuse.exe unmount <mount path>

Cloudfuse supports mounting any number of buckets.

If the container is not automatically mounted on user login after a reboot, you may need to (re)install the Cloudfuse startup program:

        cloudfuse.exe service install

To uninstall the Cloudfuse startup program use the uninstall command.

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
