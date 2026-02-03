{
  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/*";
    flake-utils.url = "github:numtide/flake-utils";
    gomod2nix = {
      url = "github:nix-community/gomod2nix";
      inputs.nixpkgs.follows = "nixpkgs";
      inputs.flake-utils.follows = "flake-utils";
    };
  };
  outputs =
    {
      self,
      flake-utils,
      nixpkgs,
      gomod2nix,
      ...
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        go = pkgs.go_1_25;
        gopls = pkgs.gopls;
        golangci-lint = pkgs.golangci-lint;
        gotest = pkgs.gotest;
        formatter = pkgs.nixfmt-tree.override {
          settings.formatter.nixfmt.includes = [ "*.nix" ];
        };
      in
      {
        packages.default = gomod2nix.legacyPackages.${system}.buildGoApplication {
          pname = "depextify";
          version = "0.1.0";
          src = ./.;
          modules = ./gomod2nix.toml;
          go = go;
        };

        legacyPackages = pkgs;

        devShells.default = pkgs.mkShellNoCC {
          packages = [
            go
            gopls
            golangci-lint
            gotest
            pkgs.actionlint

            pkgs.nil
            formatter
            gomod2nix.legacyPackages.${system}.gomod2nix
          ];
        };

        inherit formatter;
      }
    );
}
