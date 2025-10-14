{lib, ...}: let
  inherit (lib) getExe;
in {
  perSystem = {self', ...}: {
    apps = {
      default = self'.apps.hyprmon;

      hyprmon = {
        type = "app";
        program = getExe self'.packages.hyprmon;
        meta.description = self'.packages.hyprmon.meta.description;
      };
    };
  };
}
