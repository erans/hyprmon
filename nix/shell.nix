{
  perSystem = {
    self',
    pkgs,
    config,
    inputs',
    ...
  }: {
    devShells = {
      default = self'.devShells.hyprmon;

      hyprmon = pkgs.mkShellNoCC {
        name = "hyprmon-dev";

        inputsFrom = [
          config.treefmt.build.devShell
          self'.packages.hyprmon
        ];

        packages = [
          inputs'.gomod2nix.packages.default
          pkgs.flake-checker
          pkgs.go-tools
          pkgs.gotools
          pkgs.mod
          pkgs.pre-commit
          self'.packages.hyprmon
        ];
      };
    };
  };
}
