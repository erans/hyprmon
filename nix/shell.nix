{lib, ...}: let
  inherit (lib) concatLists;
in {
  perSystem = {
    self',
    pkgs,
    config,
    inputs',
    ...
  }: {
    devShells = {
      default = self'.devShells.hyprmon;

      hyprmon = pkgs.mkShell {
        name = "hyprmon-dev";

        inputsFrom = concatLists [
          (with config; [
            flake-root.devShell
            treefmt.build.devShell
          ])

          [inputs'.deadnix.devShells]
        ];

        packages = concatLists [
          (with pkgs; [
            go_1_25
            go-tools
            gotools
            mod
            pre-commit
          ])

          (with inputs'; [
            alejandra.packages.default
            gomod2nix.packages.default
          ])

          [
            config.packages.deadnix
            self'.packages.hyprmon
          ]
        ];
      };
    };
  };
}
