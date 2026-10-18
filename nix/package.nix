# ddm, built from ./ddm. The server runs this; bin/ddm builds the same source
# for local use.
{
  lib,
  buildGoModule,
}:
buildGoModule {
  pname = "ddm";
  version = "0.1.0";
  src = lib.fileset.toSource {
    root = ../ddm;
    fileset = ../ddm;
  };
  vendorHash = "sha256-g+yaVIx4jxpAQ/+WrGKxhVeliYx7nLQe/zsGpxV4Fn4=";
  subPackages = ["cmd/ddm"];
  # The tests need git and run spec checks against the repo's sites.yaml,
  # which isn't in this source; `go -C ddm test ./...` runs them.
  doCheck = false;
  meta.mainProgram = "ddm";
}
