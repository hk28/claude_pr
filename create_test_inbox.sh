#!/usr/bin/env bash
# Creates dummy inbox files/folders for testing the move pipeline.
# Run from the project root.
set -euo pipefail

AUDIO=testfolder/inbox/audio
EBOOK=testfolder/inbox/ebook

# Print 2 distinct random integers in [lo, hi]
rand2() {
    local lo=$1 hi=$2 range a b
    range=$(( hi - lo + 1 ))
    a=$(( lo + RANDOM % range ))
    b=$a
    while [ "$b" -eq "$a" ]; do
        b=$(( lo + RANDOM % range ))
    done
    echo "$a $b"
}

echo "=== Pr.Heft (audio + ebook) ==="
read -r n1 n2 <<< "$(rand2 3301 3360)"
for n in $n1 $n2; do
    pad=$(printf '%04d' "$n")
    dir="$AUDIO/Pr.Heft/pr $pad"
    mkdir -p "$dir"
    touch "$dir/pr $pad.mp3"
    echo "  audio  $dir"

    dir="$EBOOK/Pr.Heft/pr $pad"
    mkdir -p "$dir"
    touch "$dir/pr $pad.epub"
    echo "  ebook  $dir"
done

echo "=== Pr.Neo (audio only) ==="
read -r n1 n2 <<< "$(rand2 301 340)"
for n in $n1 $n2; do
    pad=$(printf '%03d' "$n")
    dir="$AUDIO/NEO 300-/neo $pad"
    mkdir -p "$dir"
    touch "$dir/neo $pad.mp3"
    echo "  audio  $dir"
done

echo "=== Heft 3150 (audio only) ==="
read -r n1 n2 <<< "$(rand2 3151 3199)"
for n in $n1 $n2; do
    pad=$(printf '%04d' "$n")
    dir="$AUDIO/Heft 3150/pr $pad"
    mkdir -p "$dir"
    touch "$dir/pr $pad.mp3"
    echo "  audio  $dir"
done

echo "=== Kartanin Miniserie (audio only) ==="
read -r n1 n2 <<< "$(rand2 1 12)"
for n in $n1 $n2; do
    pad=$(printf '%03d' "$n")
    dir="$AUDIO/Kartanin Miniserie/kartanin $pad"
    mkdir -p "$dir"
    touch "$dir/kartanin $pad.mp3"
    echo "  audio  $dir"
done

echo "Done."
