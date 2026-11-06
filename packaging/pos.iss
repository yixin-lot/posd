; 沪上·真茶铺 POS 安装脚本（Inno Setup 6）
; 构建：ISCC.exe packaging\pos.iss /DMyAppVersion=0.1.0
; 产物：release\沪上真茶铺POS-<version>-setup.exe
;
; 说明：单机版是单文件 posd.exe（内嵌前端）。安装只做三件事：
;   1) 拷文件到 Program Files；2) 建开始菜单/桌面快捷方式；3) 开机自启。

#ifndef MyAppVersion
  #define MyAppVersion "0.1.0"
#endif
#define MyAppName "Posd"
#define MyAppExeName "posd.exe"

[Setup]
AppId={{7F2C1B44-9C1E-4E7B-9E4F-2D6C8B1A5E33}
AppName={#MyAppName}
AppPublisher=上海弈新信息科技有限公司
AppComments=开源的离线单机收银系统
AppVersion={#MyAppVersion}
DefaultDirName={autopf}\Posd
DefaultGroupName={#MyAppName}
DisableProgramGroupPage=yes
OutputDir=..\release
OutputBaseFilename=pos-{#MyAppVersion}-setup
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
; 安装包自身、向导窗口、控制面板卸载项都用同一个品牌图标
SetupIconFile=pos.ico
ArchitecturesAllowed=x64compatible
ArchitecturesInstallIn64BitMode=x64compatible
PrivilegesRequired=admin
; 允许免管理员安装（/CURRENTUSER），装到 %LOCALAPPDATA%\Programs：
; 这样没有管理员权限的店员也能自己装，测试也能静默跑。
PrivilegesRequiredOverridesAllowed=dialog commandline
UninstallDisplayIcon={app}\{#MyAppExeName}

[Languages]
Name: "chinesesimplified"; MessagesFile: "compiler:Languages\ChineseSimplified.isl"

[Tasks]
Name: "autostart"; Description: "开机自动启动（收银机推荐）"; GroupDescription: "启动方式："

[Files]
Source: "..\dist\posd.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\docs\user-guide.md"; DestDir: "{app}\docs"; Flags: ignoreversion
; Apache-2.0 §4(a)：分发必须附带许可证与 NOTICE
Source: "..\release\pos-{#MyAppVersion}\LICENSE.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\release\pos-{#MyAppVersion}\NOTICE.txt"; DestDir: "{app}"; Flags: ignoreversion
Source: "..\release\pos-{#MyAppVersion}\TRADEMARK.txt"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{group}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; \
  IconFilename: "{app}\{#MyAppExeName}"
Name: "{autodesktop}\{#MyAppName}"; Filename: "{app}\{#MyAppExeName}"; \
  IconFilename: "{app}\{#MyAppExeName}"

[Registry]
; 自启要按安装模式分开放：
;   管理员安装（Program Files）→ HKLM，全机器生效
;   免管理员安装（%LOCALAPPDATA%）→ HKCU，只对当前用户生效
; 写错位置会在免管理员安装时拒绝访问、整个安装被回滚。
Root: HKLM; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
  ValueName: "PosD"; ValueData: """{app}\{#MyAppExeName}"" --addr 0.0.0.0:8080"; \
  Flags: uninsdeletevalue; Tasks: autostart; Check: IsAdminInstallMode
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; \
  ValueName: "PosD"; ValueData: """{app}\{#MyAppExeName}"" --addr 0.0.0.0:8080"; \
  Flags: uninsdeletevalue; Tasks: autostart; Check: not IsAdminInstallMode

[Run]
Filename: "{app}\{#MyAppExeName}"; Parameters: "--addr 0.0.0.0:8080"; Description: "立即启动收银服务"; Flags: nowait postinstall skipifsilent
