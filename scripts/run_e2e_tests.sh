#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Automated E2E Test Suite Runner for Event Ticket Booking System
# ==============================================================================

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

echo "================================================================="
echo " Starting Event Ticket Booking System E2E Test Execution"
echo " Project Root : ${PROJECT_ROOT}"
echo " Script Dir   : ${SCRIPT_DIR}"
echo "================================================================="

# Check if Python 3 is available
if ! command -v python3 &>/dev/null; then
    echo "[-] Error: python3 is required to run the E2E verification harness." >&2
    exit 1
fi

# Optional Docker Compose status check
if command -v docker-compose &>/dev/null && [ -f "${PROJECT_ROOT}/docker-compose.yml" ]; then
    echo "[*] Docker Compose environment detected. Checking service status:"
    (cd "${PROJECT_ROOT}" && docker-compose ps 2>/dev/null || true)
fi

# Execute Python E2E verification harness with forwarded arguments
echo "[*] Launching Python 3 E2E test harness..."
python3 "${SCRIPT_DIR}/verify_e2e.py" "$@"

EXIT_CODE=$?

if [ ${EXIT_CODE} -eq 0 ]; then
    echo "================================================================="
    echo " [✓] Test execution finished successfully."
    echo "================================================================="
else
    echo "================================================================="
    echo " [✗] Test execution failed with exit code ${EXIT_CODE}."
    echo "================================================================="
fi

exit ${EXIT_CODE}
