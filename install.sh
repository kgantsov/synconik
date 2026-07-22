#!/usr/bin/env sh
#
# synconik installer
#
# Detects OS/ARCH, downloads the matching release binary from GitHub, installs it
# to a bin directory, and sets up a managed service:
#   - Linux : a systemd service  (systemctl start/stop/status synconik, journalctl -u synconik)
#   - macOS : a launchd daemon    (launchctl / the `synconik-service` helper)
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/kgantsov/synconik/main/install.sh | sh
#
# Environment overrides:
#   VERSION       release tag to install (default: latest, e.g. VERSION=v1.2.3)
#   INSTALL_DIR   where the binary goes (default: /usr/local/bin)
#   LOG_MAX_MB    hard cap on disk used by this service's logs (default: 250)
#   NO_SERVICE=1  install the binary only, skip service setup
#
set -eu

REPO="kgantsov/synconik"
BIN_NAME="synconik"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
VERSION="${VERSION:-latest}"
NO_SERVICE="${NO_SERVICE:-0}"
LOG_MAX_MB="${LOG_MAX_MB:-250}"

# ---------------------------------------------------------------------------
# helpers
# ---------------------------------------------------------------------------
info()  { printf '\033[0;34m==>\033[0m %s\n' "$*"; }
warn()  { printf '\033[0;33mwarning:\033[0m %s\n' "$*" >&2; }
err()   { printf '\033[0;31merror:\033[0m %s\n' "$*" >&2; exit 1; }

have()  { command -v "$1" >/dev/null 2>&1; }

# Run a command as root, using sudo only when we are not already root.
as_root() {
  if [ "$(id -u)" -eq 0 ]; then
    "$@"
  elif have sudo; then
    sudo "$@"
  else
    err "this step needs root privileges but 'sudo' is not available: $*"
  fi
}

cleanup() { [ -n "${TMPDIR_INSTALL:-}" ] && rm -rf "$TMPDIR_INSTALL"; }
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# detect platform
# ---------------------------------------------------------------------------
detect_platform() {
  OS="$(uname -s)"
  ARCH="$(uname -m)"

  case "$OS" in
    Linux)  GOOS="linux" ;;
    Darwin) GOOS="darwin" ;;
    *) err "unsupported operating system: $OS" ;;
  esac

  case "$ARCH" in
    x86_64|amd64)  GOARCH="amd64" ;;
    arm64|aarch64) GOARCH="arm64" ;;
    *) err "unsupported architecture: $ARCH" ;;
  esac

  info "Detected platform: ${GOOS}/${GOARCH}"
}

# ---------------------------------------------------------------------------
# resolve version
# ---------------------------------------------------------------------------
resolve_version() {
  if [ "$VERSION" != "latest" ]; then
    return
  fi
  info "Resolving latest release..."
  api="https://api.github.com/repos/${REPO}/releases/latest"
  if have curl; then
    body="$(curl -fsSL "$api")" || err "failed to query GitHub releases API"
  elif have wget; then
    body="$(wget -qO- "$api")" || err "failed to query GitHub releases API"
  else
    err "need either 'curl' or 'wget' installed"
  fi
  VERSION="$(printf '%s' "$body" | grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')"
  [ -n "$VERSION" ] || err "could not determine the latest version"
  info "Latest release is ${VERSION}"
}

# ---------------------------------------------------------------------------
# download + install binary
# ---------------------------------------------------------------------------
download() {
  url="$1"; out="$2"
  if have curl; then
    curl -fSL --progress-bar "$url" -o "$out"
  elif have wget; then
    wget -q --show-progress -O "$out" "$url"
  else
    err "need either 'curl' or 'wget' installed"
  fi
}

install_binary() {
  # version without the leading "v" is used in the asset filename
  ver_num="${VERSION#v}"
  asset="${BIN_NAME}-${ver_num}-${GOOS}-${GOARCH}.tar.gz"
  url="https://github.com/${REPO}/releases/download/${VERSION}/${asset}"

  TMPDIR_INSTALL="$(mktemp -d)"
  info "Downloading ${asset}..."
  download "$url" "${TMPDIR_INSTALL}/${asset}"

  info "Extracting..."
  tar -xzf "${TMPDIR_INSTALL}/${asset}" -C "$TMPDIR_INSTALL"
  [ -f "${TMPDIR_INSTALL}/${BIN_NAME}" ] || err "binary '${BIN_NAME}' not found in archive"

  info "Installing to ${INSTALL_DIR}/${BIN_NAME}..."
  if [ -w "$INSTALL_DIR" ] || { [ ! -e "$INSTALL_DIR" ] && [ -w "$(dirname "$INSTALL_DIR")" ]; }; then
    mkdir -p "$INSTALL_DIR"
    install -m 0755 "${TMPDIR_INSTALL}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
  else
    as_root mkdir -p "$INSTALL_DIR"
    as_root install -m 0755 "${TMPDIR_INSTALL}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
  fi

  info "Installed $("${INSTALL_DIR}/${BIN_NAME}" --help >/dev/null 2>&1 && echo "${BIN_NAME} ${VERSION}" || echo "${BIN_NAME} ${VERSION}")"
}

# ---------------------------------------------------------------------------
# config template (created only if missing so we never clobber real settings)
# ---------------------------------------------------------------------------
write_config_template() {
  cfg_dir="$1"; cfg_file="$2"; data_dir="$3"
  if [ -f "$cfg_file" ]; then
    info "Keeping existing config at ${cfg_file}"
    return
  fi
  info "Writing config template to ${cfg_file} (edit this before starting the service)"
  as_root mkdir -p "$cfg_dir" "$data_dir"
  as_root sh -c "cat > '$cfg_file'" <<EOF
# synconik configuration -- fill in the required values, then start the service.
iconik:
  url: "https://app.iconik.io"
  app_id: ""             # REQUIRED
  token: ""              # REQUIRED
  storage_id: ""         # REQUIRED: cloud storage (GCS/S3/B2) assets upload to
  local_storage_id: ""   # REQUIRED: local ("FILE" method) storage mirroring scanner.dir
  # collection_id: ""    # optional: nest everything under an existing collection

store:
  data_dir: "${data_dir}"

scanner:
  dir: ""          # REQUIRED: absolute path of the directory to watch
  interval: 30
  stability_window: 30

uploader:
  workers: 5

log:
  level: "info"
EOF
}

# ---------------------------------------------------------------------------
# systemd service (Linux)
# ---------------------------------------------------------------------------
setup_systemd() {
  cfg_dir="/etc/synconik"
  cfg_file="${cfg_dir}/config.yaml"
  data_dir="/var/lib/synconik"
  unit="/etc/systemd/system/synconik.service"

  write_config_template "$cfg_dir" "$cfg_file" "${data_dir}/db"

  # Send this service's logs to a dedicated journald namespace so we can cap its
  # size independently of the rest of the system. journald enforces the cap
  # continuously (it vacuums old entries in real time), so a chatty 30s scan loop
  # can never fill the disk -- it just rolls over within the LOG_MAX_MB budget.
  jconf="/etc/systemd/journald@synconik.conf"
  info "Capping synconik logs at ${LOG_MAX_MB}M (journald namespace) via ${jconf}"
  as_root sh -c "cat > '$jconf'" <<EOF
[Journal]
Storage=persistent
SystemMaxUse=${LOG_MAX_MB}M
# keep individual rotated files small so the cap is honoured smoothly
SystemMaxFileSize=$(( LOG_MAX_MB / 5 ))M
EOF

  info "Installing systemd unit at ${unit}"
  as_root sh -c "cat > '$unit'" <<EOF
[Unit]
Description=synconik - watch a directory and upload files to Iconik
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=${INSTALL_DIR}/${BIN_NAME} --config ${cfg_file}
# WorkingDirectory is the config dir so ./config.yaml also resolves, making this
# work even with older binaries where the --config flag isn't honoured.
WorkingDirectory=${cfg_dir}
Restart=on-failure
RestartSec=5
# route logs into the size-capped namespace configured above
LogNamespace=synconik
# hardening
NoNewPrivileges=true
ProtectSystem=full

[Install]
WantedBy=multi-user.target
EOF

  as_root systemctl daemon-reload
  as_root systemctl restart systemd-journald@synconik.service 2>/dev/null || true

  cat <<EOF

$(printf '\033[0;32mDone.\033[0m') synconik is installed as a systemd service.
Logs are capped at ${LOG_MAX_MB} MB total (journald namespace "synconik").

Next steps:
  1. Edit the config:      sudo \$EDITOR ${cfg_file}
  2. Enable at boot:       sudo systemctl enable synconik
  3. Start it:             sudo systemctl start synconik
  4. Check status:         sudo systemctl status synconik
  5. Follow the logs:      sudo journalctl --namespace synconik -u synconik -f

Stop / restart with:       sudo systemctl stop|restart synconik
EOF
}

# ---------------------------------------------------------------------------
# launchd daemon (macOS)
# ---------------------------------------------------------------------------
setup_launchd() {
  cfg_dir="/usr/local/etc/synconik"
  cfg_file="${cfg_dir}/config.yaml"
  data_dir="/usr/local/var/synconik"
  log_dir="/usr/local/var/log"
  label="io.iconik.synconik"
  plist="/Library/LaunchDaemons/${label}.plist"

  write_config_template "$cfg_dir" "$cfg_file" "${data_dir}/db"
  as_root mkdir -p "$log_dir"

  info "Installing launchd daemon at ${plist}"
  as_root sh -c "cat > '$plist'" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${label}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${INSTALL_DIR}/${BIN_NAME}</string>
        <string>--config</string>
        <string>${cfg_file}</string>
    </array>
    <!-- WorkingDirectory is the config dir so ./config.yaml also resolves, making
         this work even with older binaries where --config isn't honoured. -->
    <key>WorkingDirectory</key>
    <string>${cfg_dir}</string>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>${log_dir}/synconik.log</string>
    <key>StandardErrorPath</key>
    <string>${log_dir}/synconik.log</string>
</dict>
</plist>
EOF
  as_root chown root:wheel "$plist"
  as_root chmod 644 "$plist"

  # launchd has no built-in log rotation, so install a tiny copytruncate rotator
  # and run it every few minutes. copytruncate (truncate the file in place) keeps
  # the daemon's open, append-mode file descriptor valid -- no restart needed.
  log_file="${log_dir}/synconik.log"
  perfile_mb=$(( LOG_MAX_MB / 5 ))
  [ "$perfile_mb" -lt 1 ] && perfile_mb=1
  rotator="${INSTALL_DIR}/synconik-logrotate"
  rot_label="${label}-logrotate"
  rot_plist="/Library/LaunchDaemons/${rot_label}.plist"

  info "Installing log rotator at ${rotator} (cap ~${LOG_MAX_MB}M)"
  as_root sh -c "cat > '$rotator'" <<EOF
#!/bin/sh
# Rotate synconik's log using copytruncate so the running daemon keeps writing.
set -eu
LOG="${log_file}"
MAX_BYTES=\$(( ${perfile_mb} * 1024 * 1024 ))
KEEP=4
[ -f "\$LOG" ] || exit 0
size=\$(stat -f%z "\$LOG" 2>/dev/null || echo 0)
[ "\$size" -gt "\$MAX_BYTES" ] || exit 0
i=\$KEEP
while [ "\$i" -gt 1 ]; do
  prev=\$(( i - 1 ))
  [ -f "\${LOG}.\${prev}.gz" ] && mv "\${LOG}.\${prev}.gz" "\${LOG}.\${i}.gz"
  i=\$prev
done
cp "\$LOG" "\${LOG}.1"
: > "\$LOG"
gzip -f "\${LOG}.1"
EOF
  as_root chmod 0755 "$rotator"

  info "Scheduling rotator via launchd (${rot_plist})"
  as_root sh -c "cat > '$rot_plist'" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN"
  "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>${rot_label}</string>
    <key>ProgramArguments</key>
    <array>
        <string>${rotator}</string>
    </array>
    <key>StartInterval</key>
    <integer>300</integer>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
EOF
  as_root chown root:wheel "$rot_plist"
  as_root chmod 644 "$rot_plist"
  as_root launchctl load -w "$rot_plist" 2>/dev/null || true

  cat <<EOF

$(printf '\033[0;32mDone.\033[0m') synconik is installed as a launchd daemon.
Logs are rotated (copytruncate, every 5 min) and capped at ~${LOG_MAX_MB} MB total.

Next steps:
  1. Edit the config:   sudo \$EDITOR ${cfg_file}
  2. Start it:          sudo launchctl load -w ${plist}
  3. Follow the logs:   tail -f ${log_file}

Stop it with:           sudo launchctl unload -w ${plist}
Restart with:           sudo launchctl unload -w ${plist} && sudo launchctl load -w ${plist}
EOF
}

# ---------------------------------------------------------------------------
# main
# ---------------------------------------------------------------------------
main() {
  detect_platform
  resolve_version
  install_binary

  if [ "$NO_SERVICE" = "1" ]; then
    info "NO_SERVICE=1 set, skipping service setup."
    info "Run manually with: ${BIN_NAME} --config /path/to/config.yaml"
    return
  fi

  case "$GOOS" in
    linux)  setup_systemd ;;
    darwin) setup_launchd ;;
  esac
}

main "$@"
