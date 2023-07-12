# Windows Support

LyveCloudFUSE supports running on Windows either as an executable or through Windows Services.

## Filename Limitations

As LyveCloudFUSE supports both Windows and Linux as well as Azure and S3 storage, there are naming restrictions that
must be followed in order for data to be available on all systems.

LyveCloudFUSE does not support filenames longer than 255 characters. It does not support filenames that contain the `\`
character or any control characters (ASCII 0 -31). On Windows it can support paths with a length longer than 255 if you
enable the LongPathsEnabled registry option. See
<https://learn.microsoft.com/en-us/windows/win32/fileio/maximum-file-path-limitation?tabs=powershell>.

If the restricted-characters-windows is enabled in S3 or Azure storage, then Windows will be able to display filenames
with the following restricted characters `<>:"|?*`. These characters are restricted on Windows, but to allow them to be
seen on Windows they will be replaced with similar looking Unicode characters.

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
