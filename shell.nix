{
  pkgs ?
    import <nixpkgs> {
      config.allowUnfree = true;
    },
  unstablePkgs ?
    import (fetchTarball "https://github.com/NixOS/nixpkgs/archive/nixos-unstable.tar.gz") {
      config.allowUnfree = true;
    },
}:
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    dotenv-cli
    air
    go
    unstablePkgs.templ
    nodejs
    git
    gnumake
    gopls
    tree
    nil
    alejandra
    jq
    watchman
  ];
  shellHook = ''
    echo "🚀 Development environment loaded."
    echo "Go version: $(go version)"
    echo "Templ version: $(templ version)"
    echo "Air version: $(air -v)"
    echo "Tailwind CSS version: $(npx tailwindcss  --help  | grep "tailwindcss v")"
  '';
}
