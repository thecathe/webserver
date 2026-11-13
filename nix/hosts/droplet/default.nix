# The droplet: everything but its disk and boot loader (hardware.nix), so the
# VM test (nix/tests/droplet.nix) runs this same configuration.
{
  config,
  lib,
  pkgs,
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
  imports = [./port-watch.nix];

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

  # Kitty's terminfo entry, so an ssh login from kitty finds the terminal.
  # Kitty hands its entry to programs it starts through $TERMINFO, but ssh
  # forwards only $TERM, and nixpkgs' ncurses has `kitty` but not `xterm-kitty`.
  # Without it zsh cannot move the cursor or set colours and redraws over itself.
  # root is the only login, so the system-wide copy is the one that matters (the
  # per-user ~/.terminfo the dots hosts use would not reach it). Same line as
  # dots' modules/nixos/terminfo.nix; this config does not import dots.
  environment.systemPackages = [pkgs.kitty.terminfo];

  nix.gc = {
    automatic = true;
    dates = "weekly";
    options = "--delete-older-than 14d";
  };
  nix.settings.auto-optimise-store = true;
}
