{
  lib,
  self,
  inputs,
  ...
}: let
  inherit (lib) substring fileContents const;

  versionRev =
    if self ? rev
    then substring 0 8 self.rev
    else "dirty";

  version = "v${fileContents (self + "/VERSION")}-${versionRev}-flake";
in {
  perSystem = {
    system,
    pkgs,
    self',
    inputs',
    ...
  }: {
    _module.args.pkgs = import inputs.nixpkgs {
      inherit system;

      overlays = with inputs; [
        gitignore.overlay
        gomod2nix.overlays.default
      ];
    };

    packages = {
      default = self'.packages.hyprmon;

      hyprmon = pkgs.buildGoApplication {
        go = pkgs.go_1_25;
        pname = "hyprmon";
        inherit version;

        subPackages = ["."];
        CGO_ENABLED = "0";

        src = pkgs.gitignoreSource ./..;
        pwd = self;
        modules = self + "/gomod2nix.toml";

        meta = {
          description = "TUI monitor configuration tool for Hyprland with visual layout, drag-and-drop, and profile management";
          homepage = "https://github.com/erans/hyprmon";
          license = lib.licenses.asl20;
          platforms = lib.platforms.linux;
          maintainers = with lib.maintainers; [onatustun];
          mainProgram = "hyprmon";
        };
      };

      deadnix =
        inputs'.deadnix.packages.default.overrideAttrs
        <| const {meta.mainProgram = "deadnix";};
    };
  };
}
