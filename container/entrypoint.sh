#!/usr/bin/env bash
set -euo pipefail

if [ "${DEBUG:-false}" = "true" ] || [ "${DEBUG:-0}" = "1" ]; then
	echo "[entrypoint] DEBUG=true: starting air with Delve (port :4444)"

	mkdir -p "${PROJECT_DIR}/build"
	cat > "${PROJECT_DIR}/build/run.sh" << 'SCRIPT'
#!/bin/sh
exec dlv exec --headless --listen=:4444 --api-version=2 --accept-multiclient --continue --log build/app --
SCRIPT
	chmod +x "${PROJECT_DIR}/build/run.sh"
	exec air
else
	if [ -x "${PROJECT_DIR}/main" ]; then
		echo "[entrypoint] DEBUG=false: starting production binary"
		exec "${PROJECT_DIR}/main"
	else
		echo "[entrypoint] production binary not found; falling back to air"
		exec air
	fi
fi
