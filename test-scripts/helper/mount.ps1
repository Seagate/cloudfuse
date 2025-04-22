# Load variables
. ".\helper\var.ps1"

# Mount step
Write-Host "Mounting into mount directory"
& "${work_dir}\cloudfuse" mount "${mount_dir}" --config-file="${work_dir}\config.yaml"

Start-Sleep -Seconds 5

# Check for mount
Write-Host "Checking for mount"
Get-Process | Where-Object { $_.Name -like "*cloudfuse*" }
