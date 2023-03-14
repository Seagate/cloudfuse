rem This batch file will download and install WinFSP
rem Usee bitsadmin to download the MSI file from the WinFSP github
bitsadmin /transfer winfsp /priority high https://github.com/winfsp/winfsp/releases/download/v2.0RC1/winfsp-2.0.23055.msi %temp%\winfsp

rem Uses msiexec to install the MSI file silently and install all developer options
msiexec /i %temp%\winfsp /quiet /norestart ADDLOCAL=ALL

rem Delete the MSI file
del %temp%\winfsp

rem Exit the script
exit