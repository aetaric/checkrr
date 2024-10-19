{ pkgs, lib, config, inputs, ... }:

{
  # https://devenv.sh/packages/
  packages = [ 
    pkgs.git
    pkgs.go
    pkgs.nodejs
    pkgs.yarn
    pkgs.ffmpeg
  ];

  languages = {
    go.enable = true;
    javascript = {
      enable = true;
      npm = { 
        enable = true;
        install.enable = true;
      };
      yarn = {
        enable = true;
        install.enable = true;
      };
    };
  };

  enterShell = ''
    go version
    node --version
    yarn --version
    ffprobe -version
  '';

  # https://devenv.sh/services/
  # services.postgres.enable = true;

  # https://devenv.sh/pre-commit-hooks/
  # pre-commit.hooks.shellcheck.enable = true;

  # See full reference at https://devenv.sh/reference/options/
}
