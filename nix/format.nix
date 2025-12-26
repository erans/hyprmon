{inputs, ...}: {
  imports = [inputs.treefmt-nix.flakeModule];

  perSystem.treefmt = {
    enableDefaultExcludes = true;
    flakeCheck = true;
    flakeFormatter = true;

    programs = {
      alejandra.enable = true;
      deadnix.enable = true;
      gofmt.enable = true;
      statix.enable = true;
    };
  };
}
