{
  inputs = {
    nixpkgs.url = "https://flakehub.com/f/NixOS/nixpkgs/*";
    flake-utils.url = "github:numtide/flake-utils";
  };
  outputs = { self, flake-utils,  nixpkgs,  ... }:
    flake-utils.lib.eachDefaultSystem (system: 
      let
        pkgs = nixpkgs.legacyPackages.${system};
        go = pkgs.go_1_25;
        gopls = pkgs.gopls;
        golangci-lint = pkgs.golangci-lint;
        gotest = pkgs.gotest;
        formatter = pkgs.nixfmt-tree.override {
          settings.formatter.nixfmt.includes = [ "*.nix"];
        };
      in
      {
          legacyPackages = pkgs;
          devShell =
            pkgs.mkShellNoCC {
              packages = [ go gopls golangci-lint gotest pkgs.nil formatter ];
            };
          inherit formatter;
      });
}
