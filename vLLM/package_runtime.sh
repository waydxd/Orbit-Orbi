#!/usr/bin/env bash
set -euo pipefail

# package_runtime.sh
# Create a tar.gz archive containing the vLLM Dockerfile and related runtime files.
# Usage:
#   ./vLLM/package_runtime.sh [output.tar.gz]
# If no output filename is provided, a timestamped archive is created in the current directory.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

OUTNAME="${1:-docker.tar}"

# Files (relative to project root) to include in the archive
FILES=(
  "vLLM/Dockerfile"
  "vLLM/entrypoint.sh"
  "vLLM/requirements.txt"
  "vLLM/common.py"
  "vLLM/worker.py"
#   "vLLM/web_server.py"
#   "vLLM/web_requirements.txt"
#   "vLLM/README.md"
#   "vLLM/DEPLOYMENT.md"
#   "vLLM/test_integration.py"
)

echo "Packaging vLLM runtime files into: $OUTNAME"

cd "$PROJECT_ROOT"

# Build list of files that actually exist
EXISTING=()
for f in "${FILES[@]}"; do
  if [ -e "$f" ]; then
    EXISTING+=("$f")
  else
    echo " - warning: $f not found, skipping"
  fi
done

if [ ${#EXISTING[@]} -eq 0 ]; then
  echo "Error: no runtime files found to package. Check that you ran this from the project root or that files exist under vLLM/." >&2
  exit 2
fi

rm "$OUTNAME"
# Create the tar archive (uncompressed) and store files at the archive root
# For each file, change to its directory and add the basename so the archive
# entries do not include the leading path (e.g. vLLM/Dockerfile -> Dockerfile).
# This preserves original filenames at the tar root.

FIRST=1
for f in "${EXISTING[@]}"; do
  dir=$(dirname "$f")
  base=$(basename "$f")
  if [ "$FIRST" -eq 1 ]; then
    # create archive with first file
    tar -C "$PROJECT_ROOT/$dir" -cf "$OUTNAME" "$base"
    FIRST=0
  else
    # append subsequent files
    tar -C "$PROJECT_ROOT/$dir" -rf "$OUTNAME" "$base"
  fi
  if [ $? -ne 0 ]; then
    echo "tar failed while adding $f" >&2
    exit 3
  fi
done

# Show resulting archive details
echo "Created $OUTNAME (size: $(wc -c < "$OUTNAME") bytes)"

# List contents
echo "Archive contents:"
if command -v tar > /dev/null 2>&1; then
  tar -tf "$OUTNAME" | sed -n '1,200p'
fi

echo "Done."
