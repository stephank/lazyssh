# Compatibility layer for Nix flakes. See `flake.nix` for documentation.
(import (fetchTarball {
  url = "https://github.com/edolstra/flake-compat/archive/5ad681b89455c3fa4591b7fbbdfb4be5990a160f.tar.gz";
  sha256 = "13jf267qvd4fvph27flp5slrn6w2q26mhpakr8bj2ppqgyjamb79";
}) { src =  ./.; }).defaultNix
