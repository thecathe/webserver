{
  description = "Server dots: ddm, the site spec, and the server's toolchains";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs?ref=nixos-unstable";
  };

  outputs = {nixpkgs, ...}: let
    pkgs = nixpkgs.legacyPackages.x86_64-linux;

    mkdocsTools = with pkgs.python3Packages; [mkdocs mkdocs-material];

    # Caddy with the Cloudflare DNS plugin, for the wildcard certificates the
    # generated Caddyfile asks for (acme_dns cloudflare). The server (M5) uses
    # the same package, so `caddy validate` here checks what will run there.
    caddy = pkgs.caddy.withPlugins {
      plugins = ["github.com/caddy-dns/cloudflare@v0.2.1"];
      hash = "sha256-jNV5COlQTKSJJk8gUZ3KEs8SGC8Z7Aiy5fk7/DvkXIo=";
    };
  in {
    packages.x86_64-linux.caddy = caddy;

    devShells.x86_64-linux = {
      # Working on this repo: ddm (Go), the docs site, and every workflow tool.
      default = pkgs.mkShell {
        buildInputs = with pkgs;
          [
            go
            gopls
            gotools
            git
            caddy # with the Cloudflare plugin, above
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
