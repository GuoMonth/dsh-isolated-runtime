# shellcheck shell=bash
# shellcheck disable=SC2034,SC2317
# Host portability only; the Cell always runs in a native Linux container.
initialize_demo_host() {
  case "$(uname -s)/$(uname -m)" in
    Linux/x86_64) demo_os=linux; demo_arch=amd64; node_arch=x64; jq_os=linux ;;
    Linux/aarch64|Linux/arm64) demo_os=linux; demo_arch=arm64; node_arch=arm64; jq_os=linux ;;
    Darwin/arm64) demo_os=darwin; demo_arch=arm64; node_arch=arm64; jq_os=macos ;;
    *) echo 'Supported hosts: Linux x86_64/arm64 and Apple Silicon macOS (native arm64 terminal)' >&2; return 1 ;;
  esac
  demo_platform="linux/$demo_arch"
  if [[ "$demo_os" == darwin ]]; then
    for required in perl shasum; do
      command -v "$required" >/dev/null || { echo "Missing prerequisite: $required" >&2; return 1; }
    done
    sha256sum() { shasum -a 256 "$@"; }
  fi
}

demo_lock() {
  if [[ "$demo_os" == darwin ]]; then
    # The inherited descriptor refers to the shell's open file description.
    # The kernel releases the lock even if demo is killed or exits unexpectedly.
    perl -MFcntl=:flock -e '
      open my $lock, ">&=9" or die "Cannot open demo lock: $!\n";
      flock($lock, $ARGV[0] eq "release" ? LOCK_UN : (LOCK_EX | LOCK_NB)) or exit 1;
    ' "$1"
  elif [[ "$1" == release ]]; then flock -u 9;
  else flock -n 9; fi
}

record_demo_process() {
  local file="$1" pid="$2"
  ps -p "$pid" -o lstart= > "$file.started"
  printf '%s\n' "$pid" > "$file"
}

stop_demo_process() {
  local file="$1" program="$2" identity="$3" pid started command_line
  [[ -f "$file" ]] || return 0
  pid="$(cat "$file")"
  if [[ "$pid" =~ ^[0-9]+$ && -f "$file.started" ]]; then
    started="$(ps -p "$pid" -o lstart= 2>/dev/null || true)"
    command_line="$(ps -ww -p "$pid" -o args= 2>/dev/null || true)"
    if [[ -n "$started" && "$started" == "$(cat "$file.started")" &&
      "$command_line" == *"$program"* && "$command_line" == *"$identity"* ]]; then
      kill "$pid" 2>/dev/null || true
      for _ in $(seq 1 50); do
        kill -0 "$pid" 2>/dev/null || break
        sleep 0.1
      done
    fi
  fi
  rm -f "$file" "$file.started"
}

require_demo_docker() {
  local engine
  command -v docker >/dev/null || { echo 'Missing prerequisite: docker (Apple Silicon: install and start Docker Desktop)' >&2; return 1; }
  engine="$(docker info --format '{{.OSType}}/{{.Architecture}}' 2>/dev/null)" || {
    echo 'Docker daemon is unavailable; start Docker Desktop or the local Linux Docker daemon' >&2; return 1;
  }
  engine="${engine/aarch64/arm64}"
  engine="${engine/x86_64/amd64}"
  [[ "$engine" == "$demo_platform" ]] || { echo "Docker must run native $demo_platform containers; found $engine" >&2; return 1; }
}
