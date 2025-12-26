{lib, ...}: {
  perSystem = {self', ...}: {
    apps = {
      default = self'.apps.hyprmon;

      hyprmon = {
        type = "app";
        program = lib.meta.getExe self'.packages.hyprmon;
        meta.description = self'.packages.hyprmon.meta.description;
      };
    };
  };
}
