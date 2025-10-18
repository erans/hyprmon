{
  perSystem = {self', ...}: {
    checks = {
      default = self'.checks.hyprmon;
      hyprmon = self'.packages.hyprmon;
    };
  };
}
