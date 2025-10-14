let
  inherit (builtins) fromJSON readFile;
  lock = fromJSON (readFile ./flake.lock);
  nodeName = lock.nodes.root.inputs.flake-compat;
  lockedInput = lock.nodes.${nodeName}.locked;
in
  import (fetchTarball {
    url = lockedInput.url or "https://github.com/edolstra/flake-compat/archive/${lockedInput.rev}.tar.gz";
    sha256 = lockedInput.narHash;
  }) {src = ./.;}
