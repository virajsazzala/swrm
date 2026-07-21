# swrm

<p align="center">
  <img src="demo.gif" alt="swrm demo" width="700">
</p>

A BitTorrent client written from scratch in Go. Bencode parsing, tracker announces (HTTP and UDP), the peer wire protocol, and piece downloading, no third-party torrent libraries. Runs as a daemon (swrmd) so downloads keep going in the background, with a CLI (swrm) to start, check, and stop them.

## Features

- HTTP and UDP tracker support, falls back across trackers if one is down
- Peer wire protocol: handshake, bitfield, choke/interest, pipelined block requests
- Resumable downloads. Restart and it just re-verifies pieces on disk (SHA-1) instead of re-downloading everything
- Reconnects with backoff if all peers drop
- Single and multi-file torrents
- Daemon and CLI split, so a download survives you closing the terminal

## Build

```bash
go build -o swrm ./cmd/swrm
go build -o swrmd ./cmd/swrmd
```

Keep both binaries in the same dir (or on your PATH). swrm looks for swrmd next to itself.

## Usage

```bash
# start and watch progress
swrm start ./some.torrent -output-dir ./downloads

# start in the background
swrm start ./some.torrent -output-dir ./downloads -d

# check on it later
swrm status -socket ~/.swrm/swrmd.sock
swrm status -socket ~/.swrm/swrmd.sock -watch

# stop it
swrm stop -socket ~/.swrm/swrmd.sock

# list all running daemons
swrm list
swrm list -clean   # remove dead sockets
```

Each start gets its own socket (default ~/.swrm/swrmd.sock, override with -socket), so you can run multiple downloads at once. If a download gets interrupted, just run swrm start again on the same output dir and it resumes.

### Flags

| command | flag | description |
|---|---|---|
| `start` | `-output-dir` | where to write files (default `.`) |
| `start` | `-socket` | socket path for the daemon |
| `start` | `-log-level` | debug / info / warn / error |
| `start` | `-log-format` | text / json |
| `start` | `-d` | detach instead of watching |
| `status` | `-watch` | live-updating progress |
| `list` | `-dir` | dir to scan for sockets |
| `list` | `-clean` | remove sockets that don't respond |

## Layout

```
cmd/swrm      CLI, talks to swrmd over a socket
cmd/swrmd     daemon that actually does the download
internal/
  bencode     bencode decoder
  torrent     .torrent parsing and validation
  tracker     HTTP and UDP tracker announce
  peer        BitTorrent wire protocol
  downloader  piece scheduling, resume, worker pool, reconnects
  daemon      daemon state machine
  api         socket API between swrm and swrmd
```

## Tests

```bash
go test ./...
```

Uses in-process fake peers and trackers for most tests, plus one real end-to-end test that builds and runs the actual binaries.