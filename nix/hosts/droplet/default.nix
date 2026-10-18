# The droplet: everything but its disk and boot loader (hardware.nix), so the
# VM test (nix/tests/droplet.nix) runs this same configuration.
{
  config,
  lib,
  ...
}: let
  # Encrypted secrets are wired in once they exist (see secrets/secrets.nix):
  # until then the system still builds, and runs without them.
  secret = name: let
    file = ../../../secrets/${name}.age;
  in
    lib.optionalAttrs (builtins.pathExists file) {${name}.file = file;};
  has = name: config.age.secrets ? ${name};
in {
  networking.hostName = "droplet";
  system.stateVersion = "26.05";
  time.timeZone = "UTC";

  services.ddm = {
    enable = true;
    # Your Tailscale login, as the admin console shows it (docs: Your to-dos).
    tailnetOwner = "thecathe@github";
    cloudflareEnvironmentFile = lib.mkIf (has "cloudflare") config.age.secrets.cloudflare.path;
    githubTokenFile = lib.mkIf (has "github-token") config.age.secrets.github-token.path;
    # Live since 2026-10-18 (D49): deploy every site that needs it, on a timer.
    autoDeploy = true;
    # The public DNS is proxied, so the web is only ever Cloudflare: nobody
    # can skip its bot blocking by using the droplet's address directly.
    webSources = import ./cloudflare-ips.nix;
  };

  age.secrets = secret "cloudflare" // secret "github-token";

  # Joined to the tailnet by hand once (`tailscale up`); the state survives
  # rebuilds in /var/lib/tailscale.
  services.tailscale.openFirewall = true;
  networking.firewall.trustedInterfaces = ["tailscale0"];

  services.openssh = {
    enable = true;
    settings = {
      PasswordAuthentication = false;
      KbdInteractiveAuthentication = false;
      PermitRootLogin = "prohibit-password";
    };
  };
  users.users.root.openssh.authorizedKeys.keys = [
    (builtins.readFile ../../../secrets/keys/cathe.pub)
  ];

  nix.gc = {
    automatic = true;
    dates = "weekly";
    options = "--delete-older-than 14d";
  };
  nix.settings.auto-optimise-store = true;
}
