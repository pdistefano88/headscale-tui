#!/usr/bin/env bash
set -euo pipefail

root_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

compose() {
  docker compose --project-directory "$root_dir" -f "$root_dir/compose.yaml" "$@"
}

headscale() {
  compose exec -T headscale headscale "$@"
}

ensure_tls() {
  local tls_dir="$root_dir/lab/tls"
  local certificate="$tls_dir/tls.crt"
  local key="$tls_dir/tls.key"

  if [[ -f $certificate && -f $key ]]; then
    return
  fi

  mkdir -p "$tls_dir"
  umask 077
  openssl req -x509 -newkey rsa:2048 -nodes -days 3650 \
    -keyout "$key" \
    -out "$certificate" \
    -subj "/CN=headscale" \
    -addext "subjectAltName=DNS:headscale" \
    >/dev/null 2>&1
}

wait_for_headscale() {
  local attempt
  for attempt in $(seq 1 30); do
    if headscale health >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done

  echo "Headscale did not become healthy" >&2
  compose logs headscale >&2
  exit 1
}

user_id() {
  local user=$1
  headscale users list --output json | jq -r --arg user "$user" '.[]? | select(.name == $user) | .id'
}

ensure_user() {
  local user=$1
  if [[ -z $(user_id "$user") ]]; then
    headscale users create "$user" --email "$user@lab.invalid" --output json >/dev/null
  fi
}

ensure_auth_key() {
  local user=$1
  local output=$2
  local id
  local key

  if [[ -f $output ]]; then
    return
  fi

  id=$(user_id "$user")
  key=$(headscale preauthkeys create --user "$id" --reusable --expiration 24h --output json | jq -r '.key')
  if [[ -z $key || $key == "null" ]]; then
    echo "Failed to generate a pre-authentication key for $user" >&2
    exit 1
  fi

  umask 077
  printf 'TS_AUTHKEY=%s\n' "$key" > "$output"
}

ensure_tailscale_image() {
  # Docker reuses its cache unless the generated certificate changed.
  compose build alice-laptop
}

wait_for_nodes() {
  local attempt
  local registered_nodes
  for attempt in $(seq 1 60); do
    registered_nodes=$(headscale nodes list --output json | jq 'length')
    if [[ $registered_nodes == 4 ]]; then
      return
    fi
    sleep 1
  done

  echo "Expected four registered Tailscale clients" >&2
  headscale nodes list --output json >&2
  compose logs >&2
  exit 1
}

disconnect_bob_phone() {
  local attempt
  local online

  compose exec -T bob-phone tailscale down >/dev/null
  for attempt in $(seq 1 30); do
    online=$(headscale nodes list --output json | jq -r \
      'first(.[]? | select(.name == "bob-phone") | .online) // false')
    if [[ $online == false ]]; then
      return
    fi
    sleep 1
  done

  echo "Bob's phone did not disconnect from Tailscale" >&2
  headscale nodes list --output json >&2
  exit 1
}

node_id() {
  local node=$1
  headscale nodes list --output json | jq -r --arg node "$node" \
    'first(.[]? | select(.givenName == $node or .name == $node) | .id) // empty'
}

ensure_tag() {
  local node=$1
  local tag=$2
  local id

  id=$(node_id "$node")
  if [[ -z $id ]]; then
    echo "Node $node was not found" >&2
    exit 1
  fi

  if headscale nodes list --output json | jq -e --arg id "$id" --arg tag "$tag" \
    'any(.[]?; (.id | tostring) == $id and ([.tags[]?] | index($tag)) != null)' \
    >/dev/null; then
    return
  fi

  headscale nodes tag --identifier "$id" --tags "$tag" --force --output json >/dev/null
}

up() {
  ensure_tls
  compose up -d headscale
  wait_for_headscale
  ensure_user alice
  ensure_user bob
  ensure_auth_key alice "$root_dir/lab/alice.env"
  ensure_auth_key bob "$root_dir/lab/bob.env"
  ensure_tailscale_image
  compose up -d alice-laptop alice-server bob-phone bob-raspberry-pi
  wait_for_nodes
  ensure_tag alice-server tag:server
  ensure_tag bob-raspberry-pi tag:raspberry-pi
  disconnect_bob_phone
}

build() {
  ensure_tls
  compose build
}

status() {
  compose ps
  headscale users list --output json
  headscale nodes list --output json
}

tui() {
  compose exec headscale headscale-tui
}

logs() {
  compose logs "$@"
}

down() {
  compose down
}

reset() {
  compose down --volumes --remove-orphans
  rm -f "$root_dir/lab/alice.env" "$root_dir/lab/bob.env"
  rm -rf "$root_dir/lab/tls"
}

case ${1:-} in
  up)
    up
    ;;
  build)
    build
    ;;
  status)
    status
    ;;
  tui)
    tui
    ;;
  logs)
    shift
    logs "$@"
    ;;
  down)
    down
    ;;
  reset)
    reset
    ;;
  *)
    echo "Usage: $0 {build|up|status|tui|logs|down|reset}" >&2
    exit 2
    ;;
esac
