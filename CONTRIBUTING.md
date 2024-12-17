# How to Contribute #

We're always looking for people to help make Checkrr even better, there are a number of ways to contribute.

## Development ##

### Tools required ###
- IDE of your choice (VS Code/Sublime Text/Atom/Goland/etc)
- [Git](https://git-scm.com/downloads)
- [NodeJS](https://nodejs.org/en/download/) (Node 20.X.X or higher)
- [pnpm](https://pnpm.io/installation)
- [Go](https://go.dev/dl/)
- [FFMpeg](https://www.ffmpeg.org/download.html)

### Contributing Code ###
- Rebase from checkrr's main branch, don't merge.
- Make meaningful commits, or squash them.
- Feel free to make a pull request before work is complete, this will let us see where it's at and make comments/suggest improvements.
- Reach out to us on the discord if you have any questions.
- Commit with *nix line endings for consistency.
- One feature/bug fix per pull request to keep things clean and easy to understand.

### Building ###
- You can build checkrr from source via this one-liner: `cd webserver && pnpm install && pnpm build && cd .. && go build`
- Please note, if you build checkrr yourself, you will be told to download the official release if you open an issue for a bug.

### Pull Requests ###
- We'll try to respond to pull requests as soon as possible, if it has been a day or two, please reach out to us, we may have missed it.
- Please keep pull requests to new features, bug fixes, or documentation. No-op changes like whitespace will be summarily closed.

### Feature Requests and Bug Reports ###
- Where possible, follow the issue template as it provides needed or useful information when fixing a bug or implementing a feature.
- Only bug reports for the latest release of checkrr will be accepted. Do not submit a bug report for a build off the main branch.