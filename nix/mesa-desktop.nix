# The Mesa desktop app from a GitHub release: Mesa.app on macOS, the
# WebKitGTK app on Linux. Nix manages its version, so the app's own update
# check and its mesa:// self-registration are turned off.
{
  lib,
  stdenv,
  stdenvNoCC,
  fetchurl,
  unzip,
  wrapGAppsHook3,
  webkitgtk_4_1,
  gtk3,
  glib,
  gdk-pixbuf,
  libsoup_3,
  release,
}:

let
  inherit (release) version;
  meta = {
    description = "The open-source SQL control plane, as a desktop app";
    homepage = "https://github.com/FelipeMalacarne/mesa";
    license = lib.licenses.agpl3Only;
    mainProgram = "mesa-desktop";
    platforms = [
      "x86_64-linux"
      "aarch64-linux"
      "x86_64-darwin"
      "aarch64-darwin"
    ];
    sourceProvenance = [ lib.sourceTypes.binaryNativeCode ];
  };
  fetch =
    file:
    fetchurl {
      url = "https://github.com/FelipeMalacarne/mesa/releases/download/v${version}/${file}";
      hash = release.hashes.${file};
    };
in
if stdenv.hostPlatform.isDarwin then
  stdenvNoCC.mkDerivation {
    pname = "mesa-sql-desktop";
    inherit version meta;
    src = fetch "Mesa_darwin_universal.zip";
    nativeBuildInputs = [ unzip ];
    sourceRoot = ".";
    # The bundle is signed; patching or stripping it would break the signature.
    dontFixup = true;
    installPhase = ''
      runHook preInstall
      mkdir -p $out/Applications $out/bin
      cp -R Mesa.app $out/Applications/
      ln -s $out/Applications/Mesa.app/Contents/MacOS/mesa-desktop $out/bin/mesa-desktop
      runHook postInstall
    '';
  }
else
  let
    libs = [
      webkitgtk_4_1
      gtk3
      glib
      gdk-pixbuf
      libsoup_3
    ];
  in
  stdenv.mkDerivation {
    pname = "mesa-sql-desktop";
    inherit version meta;
    src = fetch "mesa-desktop_linux_${if stdenv.hostPlatform.isAarch64 then "arm64" else "amd64"}.tar.gz";
    sourceRoot = "mesa-desktop_${version}_linux_${if stdenv.hostPlatform.isAarch64 then "arm64" else "amd64"}";
    nativeBuildInputs = [ wrapGAppsHook3 ];
    buildInputs = libs;
    # patchelf cannot add a RUNPATH to this Go binary without breaking its
    # program headers (it segfaults at start), so only the interpreter is set
    # and the wrapper puts the libraries on LD_LIBRARY_PATH.
    dontPatchELF = true;
    dontStrip = true;
    installPhase = ''
      runHook preInstall
      install -Dm755 mesa-desktop $out/bin/mesa-desktop
      patchelf --set-interpreter ${stdenv.cc.bintools.dynamicLinker} $out/bin/mesa-desktop
      install -Dm644 mesa.desktop $out/share/applications/mesa.desktop
      install -Dm644 mesa.png $out/share/pixmaps/mesa.png
      runHook postInstall
    '';
    preFixup = ''
      gappsWrapperArgs+=(
        --prefix LD_LIBRARY_PATH : ${lib.makeLibraryPath libs}
        --set-default MESA_UPDATE_CHECK off
        --set-default MESA_REGISTER_SCHEME off
      )
    '';
  }
