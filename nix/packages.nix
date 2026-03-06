{
  lib,
  self,
  ...
}: let
  goVersion = let
    lines = lib.strings.splitString "\n" (lib.strings.readFile (self + "/go.mod"));
    goLine = lib.lists.elemAt lines 2;
    matchResult = lib.strings.match "go ([0-9]+\\.[0-9]+)(\\.[0-9]+)?" goLine;
  in
    if matchResult == null
    then null
    else lib.lists.elemAt matchResult 0;

  goAttr =
    if goVersion == null
    then "go"
    else "go_" + lib.strings.replaceStrings ["."] ["_"] goVersion;

  root = ./..;
in {
  perSystem = {
    pkgs,
    self',
    inputs',
    ...
  }: {
    packages = {
      default = self'.packages.hyprmon;

      hyprmon = inputs'.gomod2nix.legacyPackages.buildGoApplication {
        pname = "hyprmon";
        version = "${self.shortRev or self.dirtyShortRev or "dev"}-${self._type}";

        go =
          if lib.attrsets.hasAttr goAttr pkgs
          then lib.attrsets.getAttr goAttr pkgs
          else pkgs.go;

        subPackages = ["."];
        CGO_ENABLED = "0";
        GOTOOLCHAIN = "local";

        src = lib.fileset.toSource {
          inherit root;

          fileset =
            lib.fileset.intersection
            (lib.fileset.gitTracked root)
            (lib.fileset.unions [
              (lib.fileset.fileFilter (file: file.hasExt "go") root)
              (root + "/go.mod")
              (root + "/go.sum")
              (root + "/gomod2nix.toml")
            ]);
        };

        pwd = self;
        modules = self + "/gomod2nix.toml";

        meta = {
          description = "TUI monitor configuration tool for Hyprland with visual layout, drag-and-drop, and profile management";
          homepage = "https://github.com/erans/hyprmon";
          license = lib.licenses.asl20;
          platforms = lib.platforms.linux;
          maintainers = [lib.maintainers.onatustun];
          mainProgram = "hyprmon";
        };
      };
    };
  };
}
