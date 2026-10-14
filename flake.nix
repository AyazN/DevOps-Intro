{
  description = "QuickNotes flake";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
  };

  outputs = { self, nixpkgs }:
    let
      system = "x86_64-linux";
      pkgs = import nixpkgs { inherit system; };
    in {
      packages.${system} = {
        quicknotes = pkgs.buildGoModule {
          pname = "quicknotes";
          version = "0.1.0";
          src = ./app;
          env.CGO_ENABLED = "0";
          vendorHash = null;
          ldflags = [ "-s" "-w" ];
        };

        docker = pkgs.dockerTools.buildLayeredImage {
          name = "quicknotes";
          tag = "latest";

          contents = [ self.packages.${system}.quicknotes ];

          fakeRootCommands = ''
            mkdir -p data
            chown 65534:65534 data
          '';
          enableFakechroot = true;

          config = {
            Entrypoint = [ "/bin/quicknotes" ];
            ExposedPorts = { "8080/tcp" = {}; };
            User = "65534:65534";
            WorkingDir = "/";
          };
        };

        default = self.packages.${system}.quicknotes;
      };

      devShells.${system}.default = pkgs.mkShell {
        packages = with pkgs; [ go gopls golangci-lint ];
      };
    };
}