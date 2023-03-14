rem This batch file will download and install WinFSP
rem Usee bitsadmin to download the MSI file from the WinFSP github
bitsadmin /transfer winfsp /priority high https://github.com/winfsp/winfsp/releases/download/v1.12.22339/winfsp-1.12.22339.msi %temp%\winfsp

rem Uses msiexec to install the MSI file silently
msiexec /i %temp%\winfsp /quiet /norestart ADDLOCAL=Core,Developer

rem Delete the MSI file
del %temp%\winfsp

rem Exit the script
exit