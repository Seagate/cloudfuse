; Script generated by the Inno Setup Script Wizard.
; SEE THE DOCUMENTATION FOR DETAILS ON CREATING INNO SETUP SCRIPT FILES!:
; https://jrsoftware.org/ishelp/index.php

#define MyAppName "Cloudfuse"
#define MyAppVersion "1.4.0"
#define MyAppPublisher "SEAGATE TECHNOLOGY LLC"
#define MyAppURL "https://github.com/Seagate/cloudfuse"
#define MyAppExeCLIName "cloudfuse.exe"
#define WinFSPInstaller "winfsp-2.0.23075.msi"

[Setup]
; NOTE: The value of AppId uniquely identifies this application. Do not use the same AppId value in installers for other applications.
; (To generate a new GUID, click Tools | Generate GUID inside the IDE.)
AppId={{C745CCCB-E042-4C42-852C-2FE1D287C38B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=..\LICENSE
; Uncomment the following line to run in non administrative install mode (install for current user only.)
;PrivilegesRequired=lowest
PrivilegesRequiredOverridesAllowed=commandline
OutputBaseFilename=cloudfuse
Compression=lzma
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
SignTool=signtool /d $q{#MyAppName} v{#MyAppVersion}$q $f
SignedUninstaller=yes
VersionInfoVersion={#MyAppVersion}
; Tell Windows Explorer to reload the environment
ChangesEnvironment=yes

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Dirs]
; Create directory in AppData/Roaming
Name: "{userappdata}\{#MyAppName}"; Flags: uninsalwaysuninstall

[Files]
Source: "..\cloudfuse.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\cfusemon.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\setup\baseConfig.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
Source: "..\sampleFileCacheConfigAzure.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
Source: "..\sampleFileCacheConfigS3.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
Source: "..\sampleFileCacheWithSASConfigAzure.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
Source: "..\sampleStreamingConfigAzure.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
Source: "..\sampleStreamingConfigS3.yaml"; DestDir: "{userappdata}\{#MyAppName}"; Flags: ignoreversion
; Deploy default config
Source: "..\sampleFileCacheConfigS3.yaml"; DestDir: "{userappdata}\{#MyAppName}"; DestName: "config.yaml"; Flags: onlyifdoesntexist

Source: "..\winfsp-2.0.23075.msi"; DestDir: "{app}"; Flags: ignoreversion
; NOTE: Don't use "Flags: ignoreversion" on any shared system files

[Registry]
Root: HKLM; Subkey: "SYSTEM\CurrentControlSet\Control\Session Manager\Environment"; \
    ValueType: expandsz; ValueName: "Path"; ValueData: "{olddata};{app}"; \
    Check: NeedsAddPath('{app}')

[Code]
function NeedsAddPath(Param: string): boolean;
var
  OrigPath: string;
begin
  if not RegQueryStringValue(HKEY_LOCAL_MACHINE,
    'SYSTEM\CurrentControlSet\Control\Session Manager\Environment',
    'Path', OrigPath)
  then begin
    Result := True;
    exit;
  end;
  { look for the path with leading and trailing semicolon }
  { Pos() returns 0 if not found }
  Result := Pos(';' + Param + ';', ';' + OrigPath + ';') = 0;
end;

var
  ResultCode: Integer;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // Install WinFSP if it is not already installed
    if not RegKeyExists(HKLM, 'SOFTWARE\WOW6432Node\WinFsp\Services') then
    begin
      if SuppressibleMsgBox('WinFSP is required for Cloudfuse. Do you want to install it now?', mbConfirmation, MB_YESNO, IDYES) = IDYES then
      begin
        if not Exec('msiexec.exe', '/qn /i "' + ExpandConstant('{app}\{#WinFSPInstaller}') + '"', '', SW_SHOW, ewWaitUntilTerminated, ResultCode) then
        begin
          SuppressibleMsgBox('Failed to run the WinFSP installer. You might need to install it manually.', mbError, MB_OK, IDOK);
        end;
      end;
    end;

    // Install the Cloudfuse Startup Tool
    if not Exec(ExpandConstant('{app}\{#MyAppExeCLIName}'), 'service install', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    begin
      SuppressibleMsgBox('Failed to install cloudfuse as a service. You may need to do this manually from the command line.', mbError, MB_OK, IDOK);
    end;
  end;
end;
