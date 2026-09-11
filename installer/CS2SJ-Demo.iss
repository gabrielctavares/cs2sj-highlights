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
DisableDirPage=auto
UsePreviousAppDir=yes
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

[Code]
const
  UninstallKey = 'Software\Microsoft\Windows\CurrentVersion\Uninstall\CS2SJ-Demo_is1';

var
  InstalledVersion: String;
  UpdateMode: Boolean;

function IsUpdate: Boolean;
begin
  Result := RegQueryStringValue(HKCU, UninstallKey, 'DisplayVersion', InstalledVersion);
end;

procedure InitializeWizard;
begin
  UpdateMode := IsUpdate;
  if UpdateMode then
  begin
    WizardForm.Caption := 'Atualizar o CS2SJ Demo';
    WizardForm.WelcomeLabel1.Caption := 'Atualizar o CS2SJ Demo';
    WizardForm.WelcomeLabel2.Caption :=
      'O CS2SJ Demo ' + InstalledVersion + ' já está instalado.' + #13#10 + #13#10 +
      'Este assistente atualizará o aplicativo para a versão {#AppVersion}. ' +
      'Suas configurações serão preservadas.';
  end;
end;

procedure CurPageChanged(CurPageID: Integer);
begin
  if not UpdateMode then
    Exit;

  WizardForm.Caption := 'Atualizar o CS2SJ Demo';
  if CurPageID = wpReady then
  begin
    WizardForm.ReadyLabel.Caption :=
      'O assistente está pronto para atualizar o CS2SJ Demo ' + InstalledVersion +
      ' para a versão {#AppVersion}.';
    WizardForm.NextButton.Caption := 'Atualizar';
  end
  else if CurPageID = wpFinished then
    WizardForm.FinishedHeadingLabel.Caption := 'Atualização concluída';
end;
