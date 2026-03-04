#!/usr/bin/env bash
set -euo pipefail

threshold=70.0
coverprofile="${1:-/tmp/json-canon.cover.out}"

go test ./... -covermode=count -coverprofile="$coverprofile" -count=1 >/tmp/json-canon.cover.log
cat /tmp/json-canon.cover.log

awk -F'[: ]' -v threshold="$threshold" '
BEGIN {
	required["github.com/lattice-substrate/json-canon/cmd/jcs-offline-replay"] = 1
	required["github.com/lattice-substrate/json-canon/cmd/jcs-offline-worker"] = 1
	required["github.com/lattice-substrate/json-canon/offline/runtime/executil"] = 1
}
/^mode:/ { next }
{
	path = $1
	pkg = path
	sub(/\/[^\/]+$/, "", pkg)
	statements = $3 + 0
	count = $4 + 0
	total[pkg] += statements
	totalStatements += statements
	if (count > 0) {
		covered[pkg] += statements
		coveredStatements += statements
	}
}
END {
	fail = 0
	printf "Coverage Threshold: %.1f%%\n", threshold
	for (pkg in required) {
		if (!(pkg in total) || total[pkg] == 0) {
			printf "FAIL %s missing coverage data\n", pkg
			fail = 1
			continue
		}
		pct = (covered[pkg] + 0) * 100 / total[pkg]
		status = "PASS"
		if (pct < threshold) {
			status = "FAIL"
			fail = 1
		}
		printf "%s %s %.1f%%\n", status, pkg, pct
	}
	if (totalStatements == 0) {
		printf "FAIL total coverage missing statement data\n"
		exit 1
	}
	totalPct = coveredStatements * 100 / totalStatements
	totalStatus = "PASS"
	if (totalPct < threshold) {
		totalStatus = "FAIL"
		fail = 1
	}
	printf "%s total %.1f%%\n", totalStatus, totalPct
	exit fail
}
' "$coverprofile"
