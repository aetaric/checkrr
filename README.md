# Checkrr
Scan your library files for corrupt media and replace the files via sonarr and radarr

[![](https://dcbadge.vercel.app/api/server/dkTfNKbEhJ)](https://discord.gg/dkTfNKbEhJ)

## Why does this exist
I've been running a media library for the past ~ 8 years migrating my library between both servers (I've had 3 so far) and filesystems (ext4 on LVM was a mistake). I've lost whole disks worth of data in the past and my library has had various problems ever since that I have never bothered to fully track down until now. 

Checkrr runs various checks (ffprobe, magic number, mimetype, and file hash on subsequent runs to drastically improve speed) on the path you specify as either `--checkPath` on the command line or as `checkpath` in the config. 

* If the file passes inspection, the hash is recorded in a bbolt flatfile DB so future runs are insanely fast on large libraries. 
* If the file fails all checks checkrr will check sonarr and/or radarr for the file removing it and requesting a new version via the correct system (assuming they are enabled... you could just run checkrr in a no-op state by setting `processsonarr: false` and `processradarr: false` in the config and then egrep the output like so `checkrr check | egrep "Hash Mismatch|not a recongized file type"` for environments that do not run either of these.)

## Installation
cli:
Grab a release from the releases page.

docker:
`docker pull ghcr.io/aetaric/checkrr:latest-amd64`

## Usage

### Generating a config
cli:
``` checkrr config ```

### Running Checkrr
cli:
``` checkrr check --config /etc/checkrr.yaml ```

docker:
``` docker run -v /path/to/checkrr.yaml:/etc/checkrr.yaml -v /path/to/media:/media -v /path/to/checkrr.db:/checkrr.db aetaric/checkrr:latest-arm64v8 check ```
replacing the architecture in the tag with the relevent architechture

compose:
```yaml
---
version: "3"

services:
  checkrr:
    container_name: checkrr
    image: aetaric/checkrr:latest
    command: check
    volumes:
      - /path/to/checkrr/config/checkrr.yaml:/etc/checkrr.yaml
      - /path/to/checkrr/config/checkrr.db:/checkrr.db
      - /path/to/media/to/scan:/media
    restart: on-failure
```

## Unknown file deletion
If you are feeling especially spicy, there is `--removeUnknownFiles` flag in both the config and on the cli. This flag is destructive. It will remove any file that isn't detected as a valid Video, Audio, Document, or plain text file. 

**Seriously** I don't recommend you run this on the first pass if at all. You are very likely to lose something you didn't expect to lose. 

Before using this flag, run checkrr and read the full output to ensure you don't nuke a file that you don't want to lose. Run it again with sonarr and/or radarr enabled.

*I am not responsible for your use of this flag. I will not help you sort out any damage you cause to your library. Issues opened around this flag's usage will be summarily closed as PEBCAK.*

## Contributions
Something something fork and PR if you have something to add to checkrr. I'm happy to review PRs as they come in.

## FAQ 
### Where's the WebUI?
Why do you need a webUI or a daemon'd process? Add it to cron on linux or as a scheduled task on windows.

### How do I add multiple paths for checkrr to check?
You can specify multiple folders to check via the config file
```yaml
checkpath:
  - /media/TV_Shows
  - /media/Movies
```
