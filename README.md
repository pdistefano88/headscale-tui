# Headscale TUI

An SSH-first terminal interface for administering a Headscale server.

The aim of this UI is to help manage a headscale server instance deployed
on a remote server harnessing the full power of the headscale cli without
exposing a web UI.

The TUI manages users, nodes, and preauth keys through the local `headscale`
CLI; API keys remain a placeholder menu entry. It is intended to run on the
Headscale host.

## Requirements

- Go 1.27 or newer
- Docker Engine with Docker Compose v2

## Run Locally

```bash
go run ./cmd/headscale-tui
```

Use the arrow keys or `j` and `k` to move through the menu. Press `Enter` on
Users, Nodes, or Preauth Keys to open that view; API Keys is a placeholder.
`q` or `Ctrl+C` exits.

In the Users screen, use the arrow keys or `j` and `k` to select a user. Press
`n` to enter a name and optional email and create a user, or `d` to delete the selected user.
Users with nodes cannot be deleted until their nodes are deleted or transferred.

In the Nodes screen, use the arrow keys or `j` and `k` to select a device.
Press `d` to delete the selected node, then `y` to confirm or `n` to cancel.

In the Preauth Keys screen, keys are grouped by user. Press `n` to create a
key for an existing user with Headscale's duration syntax, such as `30m`,
`24h`, or `90d`. The default is Headscale's `1h`; `d` destroys the selected
key. A newly created key is displayed until you press `Enter`; copy it then,
as Headscale only reveals its full value once. Expiration timestamps include
the TUI process timezone. Use `Tab` to reach the reusable and ephemeral
checkboxes and `Space` to toggle them.

## Development Lab

The Compose lab starts Headscale `v0.29.3` and four userspace-mode Tailscale
`v1.102.3` clients:

```text
alice
  alice-laptop
  alice-server

bob
  bob-phone
  bob-raspberry-pi
```

The seeded `alice-server` and `bob-raspberry-pi` nodes are assigned
`tag:server` and `tag:raspberry-pi`, respectively. After registration,
`bob-phone` runs `tailscale down` so it remains an offline, untagged device
owned by Bob.

Start, seed, and wait for the lab:

```bash
make lab-up
```

After changing the TUI code, rebuild the lab image before opening it again:

```bash
make lab-build
make lab-up
```

Open the TUI inside the Headscale container:

```bash
make lab-tui
```

Open the TUI inside the Headscale container after building the images and starting the
docker compose cluster:

```bash
make lab-full
```

Inspect the live users and nodes:

```bash
make lab-status
```

Other controls:

```bash
make lab-logs
make lab-down
make lab-reset
```

`lab-reset` removes all Compose volumes and generated lab credentials, then
the next `make lab-up` creates a fresh environment. The generated credentials
are stored in `lab/alice.env` and `lab/bob.env` with owner-only permissions and
are ignored by Git.

The lab exposes Headscale only at `127.0.0.1:8080`. `lab-up` generates a
self-signed certificate for the internal `headscale` DNS name and installs it
in the client image. Its Tailscale clients use userspace networking, which is
enough to exercise registration and live node state without requiring
`/dev/net/tun` or `NET_ADMIN`. It does not yet test peer traffic, subnet
routing, or exit nodes.

## Verify

```bash
make test
make vet
docker compose config
```
