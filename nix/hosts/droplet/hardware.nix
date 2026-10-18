# The droplet's disk and boot loader: a DigitalOcean KVM guest with one disk,
# partitioned by disko when nixos-anywhere installs it. GRUB boots it whether
# the droplet starts in BIOS or UEFI mode.
{modulesPath, ...}: {
  imports = [(modulesPath + "/profiles/qemu-guest.nix")];

  boot.loader.grub = {
    efiSupport = true;
    efiInstallAsRemovable = true;
  };

  # DigitalOcean gives this droplet static addresses (its cloud-init config,
  # read 2026-10-18), not DHCP. The interface is matched by MAC, which is fixed
  # for the droplet's life; a new droplet needs these values re-read.
  networking.useDHCP = false;
  networking.useNetworkd = true;
  systemd.network.networks."10-public" = {
    matchConfig.MACAddress = "ee:45:72:ed:fe:bd";
    # The public address, and the anchor address a reserved IP would use.
    address = ["64.227.40.69/20" "10.16.0.5/16"];
    gateway = ["64.227.32.1"];
    dns = ["67.207.67.2" "67.207.67.3"];
  };

  # Builds need headroom on a small droplet. The OCaml pool's first build
  # compiles Rocq's standard library, which ran out of 2 GiB RAM + 2 GiB swap
  # (2026-10-18); later builds reuse that switch (about 600 MiB, D34).
  swapDevices = [
    {
      device = "/var/swapfile";
      size = 4096;
    }
  ];

  disko.devices.disk.main = {
    type = "disk";
    device = "/dev/vda";
    content = {
      type = "gpt";
      partitions = {
        boot = {
          size = "1M";
          type = "EF02"; # GRUB's BIOS boot partition
        };
        ESP = {
          size = "512M";
          type = "EF00";
          content = {
            type = "filesystem";
            format = "vfat";
            mountpoint = "/boot";
            mountOptions = ["umask=0077"];
          };
        };
        root = {
          size = "100%";
          content = {
            type = "filesystem";
            format = "ext4";
            mountpoint = "/";
          };
        };
      };
    };
  };
}
