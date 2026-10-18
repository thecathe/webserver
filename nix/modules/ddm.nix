# services.ddm: the deployment manager on the server, and the Caddy that
# serves what it publishes.
#
# - Caddy runs the Caddyfile that `ddm caddyfile` generates from the spec, at
#   build time: a change to sites.yaml reaches Caddy through a rebuild of the
#   system (ddm-rebuild.service), never by ddm rewriting files at runtime.
# - ddm-deploy.service publishes every site (`ddm deploy`) on a timer. It is
#   the stand-in for the daemon (M6), which will poll branches instead.
# - ddm-rebuild.service switches the system to the latest config from GitHub.
#   Nothing starts it yet; the daemon will (M6).
{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.services.ddm;
  inherit (lib) mkOption mkEnableOption mkIf types;

  # The spec's default paths (spec.Paths).
  www = "/var/www";
  state = "/var/lib/ddm";

  caddyfile =
    pkgs.runCommand "Caddyfile" {
      nativeBuildInputs = [cfg.package cfg.caddyPackage];
    } ''
      ddm caddyfile --spec ${cfg.spec} --target prod > $out
      # Checks every directive and plugin. The token only has to look like
      # one: validating never calls Cloudflare.
      CLOUDFLARE_API_TOKEN=${lib.strings.replicate 40 "x"} HOME=$TMPDIR caddy validate --adapter caddyfile --config $out
    '';

  # git and nix see the GitHub token (private repos) through their own
  # config, so it never appears on a command line. The server has no SSH, so
  # GitHub SSH URLs (e.g. in a repo's .gitmodules) are fetched over https.
  deploy = pkgs.writeShellScript "ddm-deploy" ''
    set -eu
    export GIT_CONFIG_COUNT=1
    export GIT_CONFIG_KEY_0=url.https://github.com/.insteadOf
    export GIT_CONFIG_VALUE_0=git@github.com:
    if [ -e "''${CREDENTIALS_DIRECTORY:-}/github-token" ]; then
      token=$(cat "$CREDENTIALS_DIRECTORY/github-token")
      export NIX_CONFIG="access-tokens = github.com=$token"
      export GIT_CONFIG_COUNT=2
      printf 'https://x-access-token:%s@github.com\n' "$token" > "$RUNTIME_DIRECTORY/git-credentials"
      export GIT_CONFIG_KEY_1=credential.https://github.com.helper
      export GIT_CONFIG_VALUE_1="store --file=$RUNTIME_DIRECTORY/git-credentials"
    fi
    exec ddm deploy --spec ${cfg.spec} --work ${state}/work --www ${www} "$@"
  '';
  # A deploy unit. One runs at a time: they share checkouts and state.json.
  unit = description: args: {
    inherit description;
    after = ["network-online.target"];
    wants = ["network-online.target"];
    path = [cfg.package config.nix.package pkgs.git pkgs.bash pkgs.coreutils pkgs.util-linux];
    serviceConfig = {
      Type = "oneshot";
      User = "ddm";
      Group = "ddm";
      ExecStart = "${pkgs.util-linux}/bin/flock ${state}/deploy.lock ${deploy} ${args}";
      StateDirectory = "ddm";
      RuntimeDirectory = "%N";
      RuntimeDirectoryMode = "0700";
      LoadCredential = lib.optional (cfg.githubTokenFile != null) "github-token:${cfg.githubTokenFile}";
      # The OCaml pool's first build takes about 6 minutes (D34).
      TimeoutStartSec = "2h";
      Nice = 10;
      # Builds can't starve Caddy, SSH or systemd: past this the kernel
      # kills a build process (swap still counts as headroom). And a killed
      # build fails only its own site: ddm records it and carries on, where
      # systemd's default would stop the whole run.
      MemoryMax = "75%";
      OOMPolicy = "continue";
    };
  };
in {
  options.services.ddm = {
    enable = mkEnableOption "ddm, the deployment manager, with Caddy in front of it";

    package = mkOption {
      type = types.package;
      description = "The ddm package.";
    };

    caddyPackage = mkOption {
      type = types.package;
      description = "Caddy, with the Cloudflare DNS plugin the generated config uses.";
    };

    spec = mkOption {
      type = types.path;
      description = ''
        The site spec. Workflows find this repo's own dev shells and scripts
        ({{system}}) next to it, so it must sit in a copy of the whole repo.
      '';
    };

    tailnetOwner = mkOption {
      type = types.str;
      default = "";
      example = "you@github";
      description = ''
        The Tailscale login let into private sites (TAILNET_OWNER). Empty lets
        nobody in: the gate fails closed.
      '';
    };

    cloudflareEnvironmentFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = ''
        A file holding CLOUDFLARE_API_TOKEN=..., for the wildcard certificates
        (DNS-01). Without it Caddy runs, but gets no certificates.
      '';
    };

    githubTokenFile = mkOption {
      type = types.nullOr types.path;
      default = null;
      description = "A GitHub token that can read the private site repos. Without it only public repos build.";
    };

    webSources = mkOption {
      type = types.nullOr (types.listOf types.str);
      default = null;
      example = ["173.245.48.0/20" "2400:cb00::/32"];
      description = ''
        The addresses (CIDR, IPv4 or IPv6) let through the firewall to Caddy's
        ports 80 and 443, e.g. only Cloudflare's proxy. null lets anyone in.
        The tailnet reaches Caddy either way, if its interface is trusted.
      '';
    };

    autoDeploy = mkOption {
      type = types.bool;
      default = false;
      description = ''
        Run ddm-deploy on a timer (2 minutes after boot, then deployInterval
        after each run ends). Off, nothing builds or publishes until someone
        starts ddm-deploy or ddm-force@<site> by hand.
      '';
    };

    deployInterval = mkOption {
      type = types.str;
      default = "10min";
      description = "How long after one deploy finishes the next one starts (systemd time span).";
    };

    caddyfile = mkOption {
      type = types.path;
      readOnly = true;
      default = caddyfile;
      description = "The Caddyfile generated from the spec (what Caddy runs, unless overridden).";
    };

    flake = mkOption {
      type = types.str;
      default = "github:thecathe/webserver#droplet";
      description = "What ddm-rebuild.service switches the system to.";
    };
  };

  config = mkIf cfg.enable {
    users.users.ddm = {
      isSystemUser = true;
      group = "ddm";
      home = state;
    };
    users.groups.ddm = {};

    systemd.tmpfiles.rules = ["d ${www} 0755 ddm ddm -"];

    # Every workflow runs in `nix develop`.
    nix.settings.experimental-features = ["nix-command" "flakes"];

    systemd.services.ddm-deploy = unit "ddm: build and publish every site that needs it" "";
    # systemctl start ddm-force@forest: rebuild one site now, whatever its state.
    systemd.services."ddm-force@" = unit "ddm: rebuild %i now" "--force %i";

    systemd.timers.ddm-deploy = mkIf cfg.autoDeploy {
      wantedBy = ["timers.target"];
      timerConfig = {
        OnBootSec = "2min";
        OnUnitInactiveSec = cfg.deployInterval;
      };
    };

    systemd.services.ddm-rebuild = {
      description = "ddm: switch the system to its latest config";
      after = ["network-online.target"];
      wants = ["network-online.target"];
      restartIfChanged = false;
      path = [config.nix.package pkgs.git config.system.build.nixos-rebuild];
      serviceConfig.Type = "oneshot";
      script = "nixos-rebuild switch --refresh --flake ${cfg.flake}";
    };

    services.caddy = {
      enable = true;
      package = cfg.caddyPackage;
      configFile = lib.mkDefault cfg.caddyfile;
      environmentFile = cfg.cloudflareEnvironmentFile;
      openFirewall = cfg.webSources == null;
    };
    systemd.services.caddy.environment.TAILNET_OWNER = cfg.tailnetOwner;

    # Otherwise only webSources reach 80 and 443. The rules join the stock
    # iptables chain before its final refuse.
    networking.firewall.extraCommands = mkIf (cfg.webSources != null) (lib.concatMapStrings (src: ''
        ${
          if lib.hasInfix ":" src
          then "ip6tables"
          else "iptables"
        } -A nixos-fw -p tcp -m multiport --dports 80,443 -s ${src} -j nixos-fw-accept
      '')
      cfg.webSources);

    # Private mounts ask services.tailscaleAuth who a client is, over the
    # socket the generated config names (caddygen.DefaultAuthSocket).
    services.tailscale.enable = true;
    services.tailscaleAuth.enable = true;
    users.users.caddy.extraGroups = [config.services.tailscaleAuth.group];
    assertions = [
      {
        assertion = config.services.tailscaleAuth.socketPath == "/run/tailscale-nginx-auth/tailscale-nginx-auth.sock";
        message = "services.ddm: the generated Caddyfile expects tailscaleAuth's default socket path";
      }
      {
        assertion = cfg.webSources == null || !config.networking.nftables.enable;
        message = "services.ddm: webSources writes iptables rules; it does not support the nftables firewall";
      }
    ];
  };
}
