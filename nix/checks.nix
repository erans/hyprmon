{
  perSystem = {self', ...}: {
    checks = {
      default = self'.checks.hyprmon;
      inherit (self'.packages) hyprmon;
    };
  };
}
