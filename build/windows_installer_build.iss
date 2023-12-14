; Script generated by the Inno Setup Script Wizard.
; SEE THE DOCUMENTATION FOR DETAILS ON CREATING INNO SETUP SCRIPT FILES!

#define MyAppName "Cloudfuse"
#define MyAppVersion "0.2.1"
#define MyAppPublisher "Seagate Technology"
#define MyAppURL "https://github.com/Seagate/cloudfuse"
#define MyAppExeName "cloudfuseGUI.exe"
#define MyAppExeCLIName "cloudfuse.exe"
#define WinFSPInstaller "winfsp-2.0.23075.msi"

[Setup]
; NOTE: The value of AppId uniquely identifies this application. Do not use the same AppId value in installers for other applications.
; (To generate a new GUID, click Tools | Generate GUID inside the IDE.)
AppId={{C745CCCB-E042-4C42-852C-2FE1D287C38B}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
;AppVerName={#MyAppName} {#MyAppVersion}
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

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"; Flags: unchecked

[Dirs]
; Create directory in AppData/Roaming
Name: "{userappdata}\{#MyAppName}"; Flags: uninsalwaysuninstall

[Files]
Source: "..\gui\dist\cloudfuseGUI_Windows\{#MyAppExeName}"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\gui\dist\cloudfuseGUI_Windows\_internal\*"; DestDir: "{app}\_internal\"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "..\cloudfuse.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\cfusemon.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\windows-startup.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\LICENSE"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleDataSetFuseConfig.json"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleFileCacheConfigAzure.yaml"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleFileCacheConfigS3.yaml"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleFileCacheWithSASConfigAzure.yaml"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleStreamingConfigAzure.yaml"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\sampleStreamingConfigS3.yaml"; DestDir: "{app}"; Flags: ignoreversion
; Deploy default config
Source: "..\sampleFileCacheConfigS3.yaml"; DestDir: "{userappdata}\{#MyAppName}"; DestName: "config.yaml"; Flags: onlyifdoesntexist

Source: "..\winfsp-2.0.23075.msi"; DestDir: "{app}"; Flags: ignoreversion
; NOTE: Don't use "Flags: ignoreversion" on any shared system files

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent

[Code]
var
  ResultCode: Integer;

procedure CurStepChanged(CurStep: TSetupStep);
begin
  if CurStep = ssPostInstall then
  begin
    // Install WinFSP if it is not already installed
    if not RegKeyExists(HKLM, 'SOFTWARE\WOW6432Node\WinFsp\Services') then
    begin
      if MsgBox('WinFSP is required for Cloudfuse. Do you want to install it now?', mbConfirmation, MB_YESNO) = idYes then
      begin
        if not Exec('msiexec.exe', '/i "' + ExpandConstant('{app}\{#WinFSPInstaller}') + '"', '', SW_SHOW, ewWaitUntilTerminated, ResultCode) then
        begin
          MsgBox('Failed to run the WinFSP installer. You might need to install it manually.', mbError, MB_OK);
        end;
      end;
    end;

    // Add cloudfuse to the path
    if not Exec('cmd.exe', '/C SETX PATH "%PATH%;' + ExpandConstant('{app}') +'"', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    begin
      MsgBox('Failed to update PATH. You may need to add the path manually to use Cloudfuse on the command line.', mbError, MB_OK);
    end;

    // Install the Cloudfuse Startup Tool
    if not Exec(ExpandConstant('{app}\{#MyAppExeCLIName}'), 'service install', '', SW_HIDE, ewWaitUntilTerminated, ResultCode) then
    begin
      MsgBox('Failed to install cloudfuse as a service. You may need to do this manually from the command line.', mbError, MB_OK);
    end;
  end;
end;

