# The droplet's disk and boot loader: a DigitalOcean KVM guest with one disk,
# partitioned by disko when nixos-anywhere installs it. GRUB boots it whether
# the droplet starts in BIOS or UEFI mode.
{modulesPath, ...}: {
  imports = [(modulesPath + "/profiles/qemu-guest.nix")];

  boot.loader.grub = {
    efiSupport = true;
    efiInstallAsRemovable = true;
  };

  # DigitalOcean hands out the public address by DHCP; checked against the
  # droplet's current network settings before the install (M7).
  networking.useDHCP = true;

  # Builds (the OCaml pool peaks at about 600 MiB, D34) need headroom on a
  # small droplet.
  swapDevices = [
    {
      device = "/var/swapfile";
      size = 2048;
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
