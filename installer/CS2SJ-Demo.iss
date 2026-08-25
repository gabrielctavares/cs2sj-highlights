#ifndef AppVersion
  #error AppVersion is required
#endif
#ifndef StageDir
  #error StageDir is required
#endif
#ifndef OutputDir
  #error OutputDir is required
#endif

[Setup]
AppId=CS2SJ-Demo
AppName=CS2SJ Demo
AppVersion={#AppVersion}
AppVerName=CS2SJ Demo {#AppVersion}
AppPublisher=CS2SJ
DefaultDirName={localappdata}\Programs\CS2SJ-Demo
DefaultGroupName=CS2SJ Demo
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
MinVersion=10.0
OutputDir={#OutputDir}
OutputBaseFilename=CS2SJ-Demo-v{#AppVersion}-Setup
SetupIconFile={#StageDir}\assets\cs2sj-logo.ico
UninstallDisplayIcon={app}\CS2SJ-Demo.exe
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
CloseApplications=force
RestartApplications=yes
VersionInfoVersion={#AppVersion}
VersionInfoProductName=CS2SJ Demo

[Languages]
Name: "brazilianportuguese"; MessagesFile: "compiler:Languages\BrazilianPortuguese.isl"

[Tasks]
Name: "desktopicon"; Description: "Criar um atalho na área de trabalho"; GroupDescription: "Atalhos adicionais:"; Flags: unchecked

[Files]
Source: "{#StageDir}\*"; DestDir: "{app}"; Flags: ignoreversion recursesubdirs createallsubdirs

[Icons]
Name: "{group}\CS2SJ Demo"; Filename: "{app}\CS2SJ-Demo.exe"; WorkingDir: "{app}"
Name: "{autodesktop}\CS2SJ Demo"; Filename: "{app}\CS2SJ-Demo.exe"; WorkingDir: "{app}"; Tasks: desktopicon

[Run]
Filename: "{app}\CS2SJ-Demo.exe"; Description: "Abrir o CS2SJ Demo"; WorkingDir: "{app}"; Flags: nowait postinstall skipifsilent
