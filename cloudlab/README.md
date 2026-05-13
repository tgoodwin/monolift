# CloudLab

Remote bare-metal build server (and eventually e2e K8s testing) via [CloudLab](https://www.cloudlab.us).

## Setup

1. **Python venv** (one-time):

   ```bash
   python3 -m venv .venv
   .venv/bin/pip install 'portal-api[cli] @ git+https://gitlab.flux.utah.edu/emulab/portal-api.git'
   ```

2. **API token**: Download from CloudLab → your username → "Portal API Token". Save to `~/Downloads/cloudlab.jwt`. Token is valid for 2 months.

3. **Environment**: The `.env` file at the repo root is gitignored and read automatically by `cl`:

   ```
   PORTAL_TOKEN_FILE=~/Downloads/cloudlab.jwt
   PORTAL_HTTP=https://boss.emulab.net:43794
   ```

4. **Put `cl` on your PATH** (optional):

   ```bash
   mkdir -p ~/bin
   ln -s ~/projects/monolift/cloudlab/cl ~/bin/cl
   ```

## Usage

```
cl ls                              List experiments
cl status <name|id>                Details + SSH connection string
cl ssh <name|id>                   SSH into the build node
cl create [name] [project] [hours] Spin up from monolift-buildserver profile
cl extend <name|id> [hours]        Extend expiration (default: 16h)
cl terminate <name|id>             Tear down (with confirmation)
cl raw <args...>                   Pass-through to portal-cli
```

Name matching is fuzzy — `cl status tgoodwin` will match an experiment named `tgoodwin-305638`.

## Profile

`profile.py` at the repo root defines the CloudLab profile. CloudLab clones this repo to `/local/repository` on the node and runs `cloudlab/setup.sh` at boot, which installs Go, Docker, and pre-fetches the Go module cache.

Hardware type is selectable at instantiation (default: `c6525-25g`).

## Files

- `profile.py` — CloudLab profile (top-level, required by CloudLab)
- `cloudlab/cl` — CLI wrapper around `portal-cli`
- `cloudlab/setup.sh` — boot-time provisioning (Go, Docker, modules)
- `cloudlab/bootstrap.sh` — K8s stack setup (Helm, Dapr, Prometheus) for future use
