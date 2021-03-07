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
  with nixpkgs.lib;
  let

    # Common options for NixOS, nix-darwin and home-manager modules.
    moduleOptions = {
      configFile = mkOption {
        description = "Configuration file to use.";
        type = types.str;
      };
    };

    # mkCmd builds the LazySSH command-line for running it as a service.
    mkCmd = system: configFile:
      [ "${self.defaultPackage.${system}}/bin/lazyssh" "-config" configFile ];

  in {

    # Stand-alone LazySSH package, for all supported systems.
    defaultPackage = mapAttrs (system: pkgs:
      pkgs.buildGoModule {
        name = "lazyssh";
        src = ./.;
        vendorSha256 = "h3YZz9TRPJgu0kHbC8D4u+uHQnBMf8VbteyoSiypjEM=";
      }
    ) nixpkgs.legacyPackages;

    # Nixpkgs overlay that defines LazySSH.
    overlay = overlaySelf: overlaySuper: {
      lazyssh = self.defaultPackage.${overlaySuper.system};
    };

    # NixOS module to run LazySSH as a systemd service.
    #
    # Note that this creates a dedicated `lazyssh` user. If you'd rather run
    # LazySSH within your personal user account, the home-manager module is
    # preferred.
    #
    # Alternatively, use the Nixpkgs overlay and manually setup the service
    # however you want.
    nixosModule = { config, pkgs, ... }:
    let cfg = config.services.lazyssh;
    in {
      options.services.lazyssh = moduleOptions;
      config = {
        users.users.lazyssh.isSystemUser = true;
        systemd.services.lazyssh = {
          description = "LazySSH";
          wantedBy = [ "multi-user.target" ];
          after = [ "network.target" ];
          serviceConfig = {
            ExecStart = escapeShellArgs (mkCmd pkgs.system cfg.configFile);
            Restart = "on-failure";
            User = "lazyssh";
          };
        };
      };
    };

    # Nix-darwin module to run LazySSH as a launchd user agent.
    darwinModule = { config, pkgs, ... }:
    let cfg = config.services.lazyssh;
    in {
      options.services.lazyssh = moduleOptions;
      config = {
        launchd.user.agents.lazyssh = {
          serviceConfig = {
            ProgramArguments = mkCmd pkgs.system cfg.configFile;
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
    homeManagerModule = { config, pkgs, ... }:
    let cfg = config.services.lazyssh;
    in {
      options.services.lazyssh = moduleOptions;
      config = {
        systemd.user.services.lazyssh = {
          Unit.Description = "LazySSH";
          Service = {
            ExecStart = escapeShellArgs (mkCmd pkgs.system cfg.configFile);
            Restart = "on-failure";
          };
          Install.WantedBy = [ "default.target" ];
        };
      };
    };

  };

}
