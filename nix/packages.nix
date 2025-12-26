{
  lib,
  self,
  inputs,
  ...
}: let
  pname = "hyprmon";
  basePath = ./..;

  goVersion = let
    matchResult =
      lib.strings.match ".\n?go ([0-9]+\.[0-9]+)."
      (lib.strings.readFile (self + "/go.mod"));
  in
    if matchResult == null
    then null
    else lib.lists.head matchResult;

  goAttr =
    if goVersion == null
    then "go"
    else "go_" + lib.strings.replaceStrings ["."] ["_"] goVersion;

  rawVersion = lib.strings.fileContents (self + "/VERSION");
  vcsSuffix = "${self.shortRev or self.dirtyShortRev or "dev"}-${self._type}";
in {
  perSystem = {
    pkgs,
    self',
    inputs',
    ...
  }: let
    go =
      if lib.attrsets.hasAttr goAttr pkgs
      then lib.attrsets.getAttr goAttr pkgs
      else pkgs.go;
  in {
    packages = {
      default = self'.packages.hyprmon;

      hyprmon = inputs'.gomod2nix.legacyPackages.buildGoApplication {
        inherit pname go;
        version = "v${rawVersion}-${vcsSuffix}";

        subPackages = ["."];
        CGO_ENABLED = "0";

        src = lib.sources.cleanSourceWith {
          src = basePath;
          name = pname + "-source";

          filter = inputs.gitignore.lib.gitignoreFilterWith {
            inherit basePath;

            extraRules = ''
              flake.lock
              flake.nix
              .github/
              nix/
              result/
              scripts/
              release.sh
              Makefile
              img/
            '';
          };
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
