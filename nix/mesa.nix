# mesa, mesa-server and mesa-mcp from a GitHub release (static Go binaries).
{
  lib,
  stdenvNoCC,
  fetchurl,
  release,
}:

let
  inherit (release) version;
  targets = {
    x86_64-linux = "linux_amd64";
    aarch64-linux = "linux_arm64";
    x86_64-darwin = "darwin_amd64";
    aarch64-darwin = "darwin_arm64";
  };
  system = stdenvNoCC.hostPlatform.system;
  target = targets.${system} or (throw "mesa: ${system} is not supported");
  file = "mesa_${version}_${target}.tar.gz";
in
stdenvNoCC.mkDerivation {
  pname = "mesa-sql";
  inherit version;

  src = fetchurl {
    url = "https://github.com/FelipeMalacarne/mesa/releases/download/v${version}/${file}";
    hash = release.hashes.${file};
  };
  sourceRoot = "mesa_${version}_${target}";

  installPhase = ''
    runHook preInstall
    install -Dm755 mesa mesa-server mesa-mcp -t $out/bin
    runHook postInstall
  '';

  meta = {
    description = "The open-source SQL control plane: CLI, HTTP service and MCP server";
    homepage = "https://github.com/FelipeMalacarne/mesa";
    license = lib.licenses.agpl3Only;
    mainProgram = "mesa";
    platforms = builtins.attrNames targets;
    sourceProvenance = [ lib.sourceTypes.binaryNativeCode ];
  };
}
