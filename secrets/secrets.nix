# agenix: who can decrypt each secret. Encrypt or edit one with
#   nix run github:ryantm/agenix -- -e secrets/<name>.age -i <your private key>
# from the secrets/ directory. The droplet decrypts with its SSH host key at
# boot; you can with your own key.
#
#   cloudflare.age    CLOUDFLARE_API_TOKEN=<token>   (Zone:DNS:Edit, cathe.dev and jpears.me)
#   github-token.age  <token>                        (read access to the private site repos)
let
  cathe = builtins.readFile ./keys/cathe.pub;
  # The droplet's host key, generated before the install (docs: Operations).
  droplet = let f = ./keys/droplet.pub; in if builtins.pathExists f then [(builtins.readFile f)] else [];
  everyone = [cathe] ++ droplet;
in {
  "cloudflare.age".publicKeys = everyone;
  "github-token.age".publicKeys = everyone;
}
