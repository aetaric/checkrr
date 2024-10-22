# Checkrr
Scan your library files for corrupt media and replace the files via sonarr and radarr

[![](https://dcbadge.vercel.app/api/server/dkTfNKbEhJ?style=flat)](https://discord.gg/dkTfNKbEhJ) ![Docker Pulls](https://img.shields.io/docker/pulls/aetaric/checkrr) ![GitHub Downloads (all assets, all releases)](https://img.shields.io/github/downloads/aetaric/checkrr/total) ![GitHub License](https://img.shields.io/github/license/aetaric/checkrr) [![GitHub Release](https://img.shields.io/github/v/release/aetaric/checkrr)](https://github.com/aetaric/checkrr/releases) ![GitHub Release Date](https://img.shields.io/github/release-date/aetaric/checkrr)


## Why does this exist
I've been running a media library since 2013, migrating my library between both servers (I've had 3 so far) and filesystems (ext4 on LVM was a mistake). I've lost whole disks worth of data in the past and my library has had various problems ever since that I have never bothered to fully track down until now. 

Checkrr runs various checks (ffprobe, magic number, mimetype, and file hash on subsequent runs to drastically improve speed) on the path you specify as `checkpath` in the config. 

* If the file passes inspection, the hash is recorded in a bbolt flatfile DB so future runs are insanely fast on large libraries. 
* If the file fails any check checkrr will connect to sonarr and/or radarr for the file, remove it, and request a new version via the correct system (assuming they are enabled... you could just run checkrr in a no-op state by setting `sonarr.process: false` and `radarr.process: false` in the config and then egrep the output like so `checkrr check | egrep "Hash Mismatch|not a recognized file type"` for environments that do not run either of these.)

## Screenshots
![Idle screenshot](./screenshots/Idle.png?raw=true)
![Running screenshot](./screenshots/Running.png?raw=true)

## Installation and running checkrr
### cli (without package manager)
* Install prerequisite packages via your package manager or by downloading the installer (for windows): ffmpeg
* Make sure ffprobe is in your $PATH var. If you installed from a Linux/macOS package manager, it is. If you are on windows, you'll need to make sure you can run ffprobe from a basic command prompt/powershell.
* Grab a release from the releases page.
* Copy the example config from the repo: `wget https://raw.githubusercontent.com/aetaric/checkrr/main/checkrr.yaml.example -O checkrr.yaml`
* Edit the config in your favorite editor. Make sure you remove any sections you aren't using. (If you aren't using influxdb 1 and/or 2 for example, you should remove the entire stats block from your config.)If you aren't sure what the minimal config file can look like, check https://raw.githubusercontent.com/aetaric/checkrr/main/checkrr.yaml.minimal. 
* To run checkrr as a daemon, use `checkrr -c /path/to/checkrr.yaml`. If you'd like checkrr to run once and then exit (useful for running in your own cron daemon) `checkrr -c /path/to/checkrr.yaml --run-once`.

### docker
YOU MUST CREATE THE CONFIG AND DB FILES BEFORE STARTING. checkrr will complain if these are directories. Docker doesn't know you want to mount a file unless it already exists.

* creating empty db file: `touch checkrr.db`
* creating a config file from the example: `wget https://raw.githubusercontent.com/aetaric/checkrr/main/checkrr.yaml.example -O checkrr.yaml`
_make sure you edit the example config from the defaults. Remove any unused sections._
While editing the example you might want to add path mappings if the path to your media differs from arr services and checkrr.


### debian/ubuntu
```
sudo wget -O /etc/apt/trusted.gpg.d/checkrr.gpg https://repo.checkrr.aetaric.ninja/checkrr.gpg
echo "deb [signed-by=/etc/apt/trusted.gpg.d/checkrr.gpg] https://repo.checkrr.aetaric.ninja/ checkrr main" | sudo tee /etc/apt/sources.list.d/checkrr.list
sudo apt update
sudo apt install checkrr
```

### Running Checkrr
cli as a daemon:
``` checkrr -c /etc/checkrr.yaml ```

cli as a one-off:
``` checkrr -c /etc/checkrr.yaml -o```

docker:
``` docker run -v /path/to/checkrr.yaml:/etc/checkrr.yaml -v /path/to/media:/media -v /path/to/checkrr.db:/checkrr.db aetaric/checkrr:latest ```

compose:
```yaml
---
version: "3"

services:
  checkrr:
    container_name: checkrr
    image: aetaric/checkrr:latest
    volumes:
      - /path/to/checkrr/config/checkrr.yaml:/etc/checkrr.yaml
      - /path/to/checkrr/config/checkrr.db:/checkrr.db
      - /path/to/media/to/scan:/media
    ports:
      - 8585:8585
    restart: on-failure
```

### unRAID using mrslaw's community applications repo
Please note the Additional Requirements on the details screen prior to pressing install. mrslaw has all the commands you need to run there.

## Upgrading to 3.1 or newer
checkrr > 3.1 has changed the way arr services are handled. Please review the example config and bring your config into compliance prior to running checkrr. With the 3.1 release checkrr supports having multiple of each arr service. So you could have 3 sonarr instances connected. Each arr config under `arr:` has a `service` key to tell checkrr what service type it is. This can be set to `sonarr`, `radarr`, or `lidarr`. Please note that if you are running on docker, you will likely want to setup path mappings for each service. checkrr will attempt to translate the paths that the arr services see when working with their APIs.

## Building
Should you want to build checkrr from source, you can do so with the following:
`cd webserver && pnpm install && pnpm build && cd .. && go build`

You need the following to build checkrr:

go version >= 1.22

nodejs version >= 22.9.0

pnpm installed via: npm install -g pnpm


Please note, if you build checkrr yourself, you will be told to download the official release if you open an issue for a bug.

## Contributions
Something something fork and PR if you have something to add to checkrr. I'm happy to review PRs as they come in.

## FAQ

### How do I add multiple paths for checkrr to check?
You can specify multiple folders to check via the config file
```yaml
checkrr:
  checkpath:
    - /media/TV_Shows
    - /media/Movies
```
