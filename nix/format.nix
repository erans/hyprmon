{inputs, ...}: {
  imports = [inputs.treefmt-nix.flakeModule];

  perSystem = {
    config,
    inputs',
    ...
  }: {
    formatter = config.treefmt.build.wrapper;

    treefmt = {
      inherit (config.flake-root) projectRootFile;
      enableDefaultExcludes = true;

      settings.global.excludes = [
        "*.envrc"
        "flake.lock"
      ];

      programs = {
        alejandra = {
          enable = true;
          package = inputs'.alejandra.packages.default;
        };

        deadnix = {
          enable = true;
          package = config.packages.deadnix;
        };

        gofmt.enable = true;
      };
    };
  };
}
