{
  description = "Webserver-dots";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = {nixpkgs, ...}: let
    pkgs = nixpkgs.legacyPackages.x86_64-linux;
  in {
    devShells.x86_64-linux.default = pkgs.mkShell {
      buildInputs = with pkgs; [
        go
        gopls
        gotools
        # goimports
        # godoc
        # delve
        pkg-config
        caddy
        xcaddy
        git
        # docs site (mkdocs.yml, docs/)
        python3Packages.mkdocs
        python3Packages.mkdocs-material
      ];
    };
  };
}
