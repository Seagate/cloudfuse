#! /usr/bin/pwsh

'$mount_dir = "Z:"' | Out-File -FilePath ".\var.ps1"
'$mount_tmp = "~\e2e-temp"' | Out-File -FilePath ".\var.ps1" -Append
'$work_dir = "~\cloudfuse"' | Out-File -FilePath ".\var.ps1" -Append
