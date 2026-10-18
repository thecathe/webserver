# port-watch: a daily summary of who knocks on the droplet's public address,
# to judge whether closing SSH to the internet or hiding the address (a
# Cloudflare tunnel, a new droplet) is worth doing. It only reads logs.
#
# - The firewall logs every connection it refuses (logRefusedConnections):
#   80/443 from outside Cloudflare means someone using the address directly.
# - sshd logs every login attempt on the open port 22.
#
# A timer writes yesterday's report to /var/lib/port-watch/<date>.txt (kept
# 90 days). On the droplet: `port-watch` prints the last 7 reports,
# `port-watch 30` the last 30, `port-watch now` the last 24 hours.
{pkgs, ...}: let
  report = pkgs.writeShellApplication {
    name = "port-watch-report";
    runtimeInputs = with pkgs; [systemd gawk coreutils gnugrep];
    text = ''
      # port-watch-report SINCE UNTIL LABEL
      since=$1 until=$2 label=$3
      fw=$(journalctl -k -o cat --since "$since" --until "$until" --grep 'refused connection: ' || true)
      ssh=$(journalctl -t sshd -t sshd-session -t sshd-auth -o cat --since "$since" --until "$until" || true)

      echo "port-watch: $label"
      echo
      echo "## Refused by the firewall (any port not open to that address)"
      # A connection counts once (the firewall logs every retried SYN), and
      # counters are kept explicitly: gawk's length() turns an array that was
      # never filled into a scalar.
      printf '%s\n' "$fw" | awk '
        /SRC=/ {
          for (i = 1; i <= NF; i++) { split($i, kv, "="); f[kv[1]] = kv[2] }
          if (seen[f["SRC"] " " f["SPT"] " " f["PROTO"] f["DPT"]]++) next
          n++; if (!src[f["SRC"]]++) ns++; port[f["PROTO"] "/" f["DPT"]]++
          if (f["DPT"] == 80 || f["DPT"] == 443) { web++; if (!websrc[f["SRC"]]++) nweb++ } }
        END {
          printf "%d connections from %d addresses\n", n, ns
          printf "80/443 directly (not through Cloudflare): %d from %d addresses\n", web, nweb
          if (n) {
            print "by port (top 10):"
            for (p in port) printf "  %7d  %s\n", port[p], p | "sort -rn | head -10"
            close("sort -rn | head -10")
            print "by address (top 5):"
            for (s in src) printf "  %7d  %s\n", src[s], s | "sort -rn | head -5" }
        }'
      echo
      echo "## SSH on port 22 (open to everyone)"
      printf '%s\n' "$ssh" | awk '
        function addr() {
          if (match($0, /[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+/)) return substr($0, RSTART, RLENGTH)
          if (match($0, / [0-9a-f]*:[0-9a-f:]+ port /)) return substr($0, RSTART + 1, RLENGTH - 7)
          return "" }
        function sport() { return match($0, / port [0-9]+/) ? substr($0, RSTART + 6, RLENGTH - 6) : "" }
        /^Accepted / { ok++; a = addr()
          # Tailscale addresses (100.64.0.0/10, fd7a:115c:a1e0::/48) are the tailnet.
          split(a, o, ".")
          if (!((a ~ /^fd7a:115c:a1e0:/) || (o[1] == 100 && o[2] >= 64 && o[2] < 128))) { out++; outside[out] = $0 }
          next }
        /Invalid user |Failed |\[preauth\]|banner exchange|kex_exchange_identification|Unable to negotiate/ {
          a = addr(); if (a == "") next
          if (match($0, /^Invalid user [^ ]+/) && !user[substr($0, 14, RLENGTH - 13)]++) nu++
          if (seen[a " " sport()]++) next
          t++; if (!tries[a]++) nt++ }
        END {
          printf "failed or abandoned attempts: %d from %d addresses\n", t, nt
          printf "logins: %d, from outside the tailnet: %d\n", ok, out
          for (i = 1; i <= out; i++) print "  OUTSIDE: " outside[i]
          if (nu) {
            print "usernames tried (top 5):"
            for (u in user) printf "  %7d  %s\n", user[u], u | "sort -rn | head -5"
            close("sort -rn | head -5") }
          if (nt) {
            print "by address (top 5):"
            for (s in tries) printf "  %7d  %s\n", tries[s], s | "sort -rn | head -5" }
        }'
    '';
  };

  cli = pkgs.writeShellApplication {
    name = "port-watch";
    runtimeInputs = [report pkgs.coreutils];
    text = ''
      if [ "''${1:-}" = now ]; then
        exec port-watch-report "-24h" "now" "the last 24 hours"
      fi
      days=''${1:-7}
      shopt -s nullglob
      files=(/var/lib/port-watch/*.txt)
      if [ ''${#files[@]} -eq 0 ]; then
        echo "no reports yet (one is written each night); try: port-watch now"
        exit 0
      fi
      for f in $(printf '%s\n' "''${files[@]}" | tail -n "$days"); do cat "$f"; echo; done
    '';
  };
in {
  networking.firewall.logRefusedConnections = true;
  environment.systemPackages = [cli];

  systemd.services.port-watch = {
    description = "port-watch: summarise yesterday's knocks on the public ports";
    path = [report pkgs.coreutils pkgs.findutils];
    serviceConfig = {
      Type = "oneshot";
      StateDirectory = "port-watch";
    };
    script = ''
      day=$(date -d yesterday +%F)
      port-watch-report "$day" "$(date +%F)" "$day" > "/var/lib/port-watch/$day.txt"
      find /var/lib/port-watch -name '*.txt' -mtime +90 -delete
    '';
  };
  systemd.timers.port-watch = {
    wantedBy = ["timers.target"];
    timerConfig = {
      OnCalendar = "*-*-* 00:10:00";
      Persistent = true;
    };
  };
}
