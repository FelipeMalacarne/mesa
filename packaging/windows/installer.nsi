; Mesa installer for Windows: a per-user install (no administrator rights)
; that adds a Start menu entry, registers mesa:// links and an uninstaller.
;   makensis /DVERSION=1.2.3 /DSOURCE=path\to\mesa-desktop.exe /DOUTFILE=Mesa_1.2.3_windows_amd64_setup.exe installer.nsi
Unicode true
RequestExecutionLevel user
SetCompressor /SOLID lzma

!ifndef VERSION
  !error "pass /DVERSION=x.y.z"
!endif
!ifndef SOURCE
  !error "pass /DSOURCE=path to mesa-desktop.exe"
!endif
!ifndef OUTFILE
  !define OUTFILE "Mesa_${VERSION}_setup.exe"
!endif

!include "MUI2.nsh"

Name "Mesa ${VERSION}"
OutFile "${OUTFILE}"
InstallDir "$LOCALAPPDATA\Programs\Mesa"
InstallDirRegKey HKCU "Software\Mesa" "InstallDir"
!define MUI_ICON "..\icons\mesa.ico"
!define MUI_UNICON "..\icons\mesa.ico"
!define MUI_FINISHPAGE_RUN "$INSTDIR\mesa-desktop.exe"

!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Mesa"

Section "Mesa"
  SetOutPath "$INSTDIR"
  File "/oname=mesa-desktop.exe" "${SOURCE}"
  File "..\icons\mesa.ico"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortcut "$SMPROGRAMS\Mesa.lnk" "$INSTDIR\mesa-desktop.exe" "" "$INSTDIR\mesa.ico"

  WriteRegStr HKCU "Software\Mesa" "InstallDir" "$INSTDIR"
  WriteRegStr HKCU "Software\Classes\mesa" "" "URL:Mesa"
  WriteRegStr HKCU "Software\Classes\mesa" "URL Protocol" ""
  WriteRegStr HKCU "Software\Classes\mesa\DefaultIcon" "" "$INSTDIR\mesa.ico"
  WriteRegStr HKCU "Software\Classes\mesa\shell\open\command" "" '"$INSTDIR\mesa-desktop.exe" "%1"'

  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "Mesa"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "Mesa"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\mesa.ico"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "InstallLocation" "$INSTDIR"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  Delete "$SMPROGRAMS\Mesa.lnk"
  Delete "$INSTDIR\mesa-desktop.exe"
  Delete "$INSTDIR\mesa-desktop.exe.old"
  Delete "$INSTDIR\mesa.ico"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  DeleteRegKey HKCU "Software\Classes\mesa"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  DeleteRegKey HKCU "Software\Mesa"
SectionEnd
