{
  pkgs ? (import <nixpkgs> {
    config.allowUnfree = true;
  }),
  ...
}:
pkgs.mkShell {
  nativeBuildInputs = with pkgs; [
    dotenv-cli
    air
    go
    templ
    tailwindcss
    nodejs
    git
    gnumake
    gopls
    tree
    nil
    alejandra
  ];
  shellHook = ''
    echo "🚀 Development environment loaded."
    echo "Go version: $(go version)"
    echo "Templ version: $(templ version)"
    echo "Air version: $(air -v)"
    echo "Tailwind CSS version: $(tailwindcss  --help  | grep "tailwindcss v")"
  '';
}
