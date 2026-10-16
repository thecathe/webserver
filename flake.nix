{
  description = "Server dots: ddm, the site spec, and the server's toolchains";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = {nixpkgs, ...}: let
    pkgs = nixpkgs.legacyPackages.x86_64-linux;

    mkdocsTools = with pkgs.python3Packages; [mkdocs mkdocs-material];
  in {
    devShells.x86_64-linux = {
      # Working on this repo: ddm (Go), the docs site, and every workflow tool.
      default = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            go
            gopls
            gotools
            git
            caddy
            python3 # scripts/generate_docs.py
          ]
          ++ mkdocsTools;
      };

      # Workflow environments (sites.yaml `env: { flake: "{{system}}#<name>" }`),
      # for site repos that don't ship a devShell of their own. Kept small so a
      # build doesn't pull in this repo's Go toolchain.
      mkdocs = pkgs.mkShell {
        buildInputs = [pkgs.python3] ++ mkdocsTools;
      };

      ocaml-docs = pkgs.mkShell {
        buildInputs = with pkgs.ocamlPackages; [
          ocaml
          dune_3
          findlib
          odoc
          ppx_deriving
          yojson
        ];
      };
    };
  };
}
