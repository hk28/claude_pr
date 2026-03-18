# prman

Perry Rhodan media manager — tracks, scrapes metadata from Perrypedia, and manages audio/ebook files across inbox and output folders.

## Running locally

**Requirements:** Go 1.25+

```bash
git clone https://github.com/hk28/claude_pr
cd claude_pr
go mod download
```

Create the local config (inbox/outbox paths for your machine):

```bash
cp config/main.yaml config-local/main.yaml
# edit config-local/main.yaml with your local inbox/outbox paths
ln -s ../config/series config-local/series   # share series configs
```

Run:

```bash
go run . -config ./config-local
```

Open [http://localhost:8080](http://localhost:8080).

### VSCode debug

A launch configuration is included in `.vscode/launch.json`. It starts the app with `-config ./config-local`. Press **F5** to launch.

Delve must be built with Go 1.25. If you see a DWARFv5 error, rebuild it:

```bash
GOROOT=~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64 \
  ~/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.25.0.linux-amd64/bin/go \
  install github.com/go-delve/delve/cmd/dlv@latest
```

---

## Docker

### Directory layout on the host

```
/your/media/
├── inbox/
│   ├── audio/       # drop audio folders here
│   └── ebook/       # drop ebook folders here
└── outbox/
    ├── audio/
    └── outbox/
```

### config/main.yaml

The included `config/main.yaml` uses the container-internal paths:

```yaml
inbox_audio: /inbox/audio
inbox_ebook: /inbox/ebook
output_audio: /outbox/audio
output_ebook: /outbox/ebook
```

### docker-compose.yml

Edit the volume mounts to point to your actual media directories, then start:

```bash
docker compose up -d
```

Example `docker-compose.yml`:

```yaml
services:
  prman:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./config:/app/config          # series configs + main.yaml
      - ./data:/app/data              # cache, state, covers (persistent)
      - /your/media/inbox/audio:/inbox/audio
      - /your/media/inbox/ebook:/inbox/ebook
      - /your/media/outbox/audio:/outbox/audio
      - /your/media/outbox/ebook:/outbox/ebook
    restart: unless-stopped
```

### Update to latest version

```bash
docker compose down
cd claude_pr && git pull && cd ..
docker compose up -d --build --no-cache
```

> **Note:** Do not use `docker build -t prman .` followed by `docker compose up -d` — Compose ignores manually tagged images unless `image: prman` is set in the compose file. Use `--build` to let Compose rebuild from the updated source.

### Run without compose

```bash
docker run -d \
  -p 8080:8080 \
  -v $(pwd)/config:/app/config \
  -v $(pwd)/data:/app/data \
  -v /your/media/inbox/audio:/inbox/audio \
  -v /your/media/inbox/ebook:/inbox/ebook \
  -v /your/media/outbox/audio:/outbox/audio \
  -v /your/media/outbox/ebook:/outbox/ebook \
  prman
```

---

## Series config

Series are defined as YAML files in `config/series/`. The filename (without `.yaml`) becomes the series slug used in URLs.

Key fields:

| Field | Description |
|---|---|
| `name` | Display name |
| `url` | Perrypedia URL template (`%d` = issue number) |
| `scanpattern` | Pattern to match inbox filenames. Use `%d` or `%.03d` for the number, or just a keyword like `neo` |
| `types` | `[audio]`, `[ebook]`, or `[audio, ebook]` |
| `states` | List of state names (e.g. `Released`, `Available`, `Read`) |
| `latest` | Latest known issue number (0 = ongoing) |
| `scanfrom` | First issue number to scrape/scan |

Example:

```yaml
name: Perry Rhodan Neo
url: https://www.perrypedia.de/wiki/Quelle:PRNEO%d
scanpattern: "neo"
types:
  - audio
  - ebook
states:
  - Released
  - Available
  - Read
```
