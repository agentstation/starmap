{
  description = "Swag v2 - OpenAPI 3.1 documentation generator";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = "2.0.0-rc4";

        platformInfo = {
          "x86_64-linux" = {
            platform = "Linux_x86_64";
            sha256 = pkgs.lib.fakeHash;
          };
          "aarch64-linux" = {
            platform = "Linux_arm64";
            sha256 = pkgs.lib.fakeHash;
          };
          "i686-linux" = {
            platform = "Linux_i386";
            sha256 = pkgs.lib.fakeHash;
          };
          "x86_64-darwin" = {
            platform = "Darwin_x86_64";
            sha256 = pkgs.lib.fakeHash;
          };
          "aarch64-darwin" = {
            platform = "Darwin_arm64";
            sha256 = "sha256-eeMsOoXkqQpO9PkE6VGjBPG/slDtVCKfNSBT/NRSyqs=";
          };
        };

        buildSwag = info:
          pkgs.stdenv.mkDerivation {
            pname = "swag";
            inherit version;

            src = pkgs.fetchurl {
              url = "https://github.com/swaggo/swag/releases/download/v${version}/swag_${version}_${info.platform}.tar.gz";
              sha256 = info.sha256;
            };

            sourceRoot = ".";

            installPhase = ''
              mkdir -p $out/bin
              cp swag $out/bin/
              chmod +x $out/bin/swag
            '';
          };

        swagForSystem =
          if builtins.hasAttr system platformInfo
          then buildSwag platformInfo.${system}
          else null;

      in {
        packages = {
          swag = swagForSystem;
          default = swagForSystem;
        };

        apps.default = flake-utils.lib.mkApp {
          drv = self.packages.${system}.default;
        };
      }
    );
}
