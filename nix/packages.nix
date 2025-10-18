{
  lib,
  self,
  inputs,
  ...
}: let
  inherit (lib) readFile match head hasAttr getAttr replaceStrings substring fileContents const;

  goVersion =
    self
    + "/go.mod"
    |> readFile
    |> match ".\n?go ([0-9]+\.[0-9]+)."
    |> (matchResult:
      if matchResult == null
      then null
      else head matchResult);

  goAttr =
    if goVersion == null
    then "go"
    else "go_" + replaceStrings ["."] ["_"] goVersion;

  versionRev =
    if self ? rev
    then substring 0 8 self.rev
    else "dirty";

  version =
    self
    + "/VERSION"
    |> fileContents
    |> (versionText: "v${versionText}-${versionRev}-flake");
in {
  perSystem = {
    system,
    pkgs,
    self',
    inputs',
    ...
  }: let
    go =
      if hasAttr goAttr pkgs
      then getAttr goAttr pkgs
      else pkgs.go;
  in {
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
        pname = "hyprmon";
        inherit go version;

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
