#!/usr/bin/env bash
# Deletes all content from the outbox folders, keeping the directories themselves.
# Run from the project root.
set -euo pipefail

OUTBOX=testfolder/outbox

if [ ! -d "$OUTBOX" ]; then
    echo "Outbox not found: $OUTBOX"
    exit 0
fi

count=0
while IFS= read -r -d '' entry; do
    rm -rf "$entry"
    echo "  removed  $entry"
    (( count++ )) || true
done < <(find "$OUTBOX" -mindepth 2 -maxdepth 2 -print0)

echo "Done. Removed $count items."
