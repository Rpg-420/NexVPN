; Inno Setup Script for NexVPN Client v1.3.5
; Produces NexVPN-Setup-v1.3.5-x64.exe

#define MyAppName "NexVPN"
#define MyAppVersion "1.3.5"
#define MyAppPublisher "NexVPN Team"
#define MyAppURL "https://github.com/Rpg-420/NexVPN"
#define MyAppExeName "NexVPN.exe"

[Setup]
AppId={{D3F48671-8B02-4A73-A3FE-94C3F0829871}
AppName={#MyAppName}
AppVersion={#MyAppVersion}
AppPublisher={#MyAppPublisher}
AppPublisherURL={#MyAppURL}
AppSupportURL={#MyAppURL}
AppUpdatesURL={#MyAppURL}
DefaultDirName={autopf}\{#MyAppName}
DisableProgramGroupPage=yes
LicenseFile=LICENSE
OutputDir=dist
OutputBaseFilename=NexVPN-Setup-v1.3.5-x64
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
PrivilegesRequired=admin
ArchitecturesInstallIn64BitMode=x64
CloseApplications=yes
RestartApplications=no

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "{cm:CreateDesktopIcon}"; GroupDescription: "{cm:AdditionalIcons}"
Name: "autostart"; Description: "Launch NexVPN automatically when Windows starts"; GroupDescription: "Startup:"

[Files]
Source: "build\bin\NexVPN.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "bin\*"; DestDir: "{app}\bin"; Flags: ignoreversion recursesubdirs createallsubdirs
Source: "geoip.dat"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "geosite.dat"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "wintun.dll"; DestDir: "{app}\bin"; Flags: ignoreversion
Source: "README.md"; DestDir: "{app}"; Flags: ignoreversion
Source: "LICENSE"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; Tasks: desktopicon

[Run]
Filename: "{app}\{#MyAppExeName}"; Description: "{cm:LaunchProgram,{#StringChange(MyAppName, '&', '&&')}}"; Flags: nowait postinstall skipifsilent
