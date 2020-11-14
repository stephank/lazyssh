# Nix flake for LazySSH.
#
# At the time of writing, Nix flakes are experimental. It's likely you're not
# on a version of Nix that supports flakes just yet. In that case, import this
# as follows:
#
# ```
# let
#   lazyssh = import (fetchTarball {
#     # Replace `<REV>` with the exact Git commit hash you want.
#     url = "https://github.com/stephank/lazyssh/archive/<REV>.tar.gz";
#     # Easiest way to find this hash is to just try a build.
#     sha256 = "0000000000000000000000000000000000000000000000000000";
#   });
# in
#   # Your code here...
# ```
#
# But if you ARE using flakes, you can use this as an input:
#
# ```
# {
#   inputs.lazyssh.url = github:stephank/lazyssh;
#   outputs = { self, lazyssh }: {
#     # Your code here...
#   };
# }
# ```
#
# In either case, you then use one of the `lazyssh` attributes. This flake
# provides a stand-alone package, a Nixpkgs overlay, and modules for NixOS,
# nix-darwin and home-manager.
#
# TODO: Declarative configuration for LazySSH.

{
  description = "LazySSH";

  inputs.nixpkgs.url = github:NixOS/nixpkgs/nixpkgs-unstable;

  outputs = { self, nixpkgs }:
    let

      # mkPackage builds the LazySSH derivation. Function over Nixpkgs.
      mkPackage = { buildGoModule, ... }:
        buildGoModule {
          name = "lazyssh";
          src = ./.;
          vendorSha256 = "F6Z/ESmSZ5v/Recp4BcAvOvwgmAlJAuf9vrelGGyjYg=";
        };

      # mkModuleOptions builds the common module options.
      # Function over the Nixpkgs library.
      mkModuleOptions = lib: with lib; {
        configFile = mkOption {
          description = "Configuration file to use.";
          type = types.str;
        };
      };

      # mkCmd builds the LazySSH command-line for running it as a service.
      # Function over Nixpkgs (with overlay) and the config file.
      mkCmd = { lazyssh, ... }: configFile:
        [ "${lazyssh}/bin/lazyssh" "-config" configFile ];

    in {

      # Stand-alone LazySSH package.
      defaultPackage = nixpkgs.lib.mapAttrs (system: pkgs:
        mkPackage pkgs
      ) nixpkgs.legacyPackages;

      # Nixpkgs overlay that defines LazySSH.
      overlay = self: super: {
        lazyssh = mkPackage super;
      };

      # NixOS module to run LazySSH as a systemd service.
      #
      # Note that this creates a dedicated `lazyssh` user. If you'd rather run
      # LazySSH within your personal user account, the home-manager module is
      # preferred.
      #
      # Alternatively, use the Nixpkgs overlay and manually setup the service
      # however you want.
      nixosModule = { config, lib, pkgs, ... }:
        let cfg = config.services.lazyssh;
        in {
          options.services.lazyssh = mkModuleOptions lib;
          config = {
            nixpkgs.overlays = [ self.overlay ];
            users.users.lazyssh.isSystemUser = true;
            systemd.services.lazyssh = {
              description = "LazySSH";
              wantedBy = [ "multi-user.target" ];
              after = [ "network.target" ];
              serviceConfig = {
                ExecStart = lib.escapeShellArgs (mkCmd pkgs cfg.configFile);
                Restart = "on-failure";
                User = "lazyssh";
              };
            };
          };
        };

      # Nix-darwin module to run LazySSH as a launchd user agent.
      darwinModule = { config, lib, pkgs, ... }:
        let cfg = config.services.lazyssh;
        in {
          options.services.lazyssh = mkModuleOptions lib;
          config = {
            nixpkgs.overlays = [ self.overlay ];
            launchd.user.agents.lazyssh = {
              serviceConfig = {
                ProgramArguments = mkCmd pkgs cfg.configFile;
                RunAtLoad = true;
                KeepAlive = true;
              };
            };
          };
        };

      # Home-manager module to run LazySSH as a systemd user service.
      #
      # This only works on Linux. On Mac, the nix-darwin module is perferred.
      #
      # Alternatively, use the Nixpkgs overlay and manually setup the service
      # however you want.
      homeManagerModule = { config, lib, pkgs, ... }:
        let cfg = config.services.lazyssh;
        in {
          options.services.lazyssh = mkModuleOptions lib;
          config = {
            nixpkgs.overlays = [ self.overlay ];
            systemd.user.services.lazyssh = {
              Unit.Description = "LazySSH";
              Service = {
                ExecStart = lib.escapeShellArgs (mkCmd pkgs cfg.configFile);
                Restart = "on-failure";
              };
              Install.WantedBy = [ "default.target" ];
            };
          };
        };

    };

}
