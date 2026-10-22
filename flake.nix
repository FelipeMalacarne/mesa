{
  description = "Mesa, the open-source SQL control plane: CLI, HTTP service, MCP server and desktop app";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

  outputs =
    { self, nixpkgs }:
    let
      # The release the packages install; nix/update.sh <version> moves it.
      release = builtins.fromJSON (builtins.readFile ./nix/release.json);
      # nixpkgs-unstable no longer builds for Intel Macs; there the overlay
      # still works on a nixpkgs release that does (26.05).
      systems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      forAllSystems = f: nixpkgs.lib.genAttrs systems (system: f nixpkgs.legacyPackages.${system});
      build = pkgs: {
        mesa = pkgs.callPackage ./nix/mesa.nix { inherit release; };
        mesa-desktop = pkgs.callPackage ./nix/mesa-desktop.nix { inherit release; };
      };
    in
    {
      packages = forAllSystems (
        pkgs:
        let
          p = build pkgs;
        in
        p // { default = p.mesa; }
      );

      # nixpkgs already has a "mesa" (the OpenGL library), so the overlay
      # uses other names.
      overlays.default =
        final: _prev:
        let
          p = build final;
        in
        {
          mesa-sql = p.mesa;
          mesa-sql-desktop = p.mesa-desktop;
        };
    };
}
