.PHONY: gate-spec gate-lint gate-test gate-conformance gate-determinism gate-build gate-all

# Gate DAG: SPEC → LINT → TEST → CONFORMANCE → DETERMINISM → BUILD

gate-spec:
	@echo "=== GATE: SPEC ==="
	@# Verify every requirement ID in REQ_REGISTRY.md appears in REQ_ENFORCEMENT_MATRIX.md
	@grep -oP '[A-Z]+-[A-Z0-9]+-[0-9]+' REQ_REGISTRY.md | sort -u > /tmp/jcs-req-ids.txt
	@grep -oP '^[A-Z]+-[A-Z0-9]+-[0-9]+' REQ_ENFORCEMENT_MATRIX.md | sort -u > /tmp/jcs-matrix-ids.txt
	@diff /tmp/jcs-req-ids.txt /tmp/jcs-matrix-ids.txt || (echo "FAIL: requirement/matrix mismatch"; exit 1)
	@echo "PASS: all requirement IDs covered in enforcement matrix"

gate-lint:
	@echo "=== GATE: LINT ==="
	go vet ./...

gate-test: gate-lint
	@echo "=== GATE: TEST ==="
	go test ./jcserr ./jcsfloat ./jcstoken ./jcs -count=1 -v

gate-conformance: gate-test
	@echo "=== GATE: CONFORMANCE ==="
	go test ./conformance -count=1 -v -timeout=10m

gate-determinism: gate-conformance
	@echo "=== GATE: DETERMINISM ==="
	go test ./conformance -count=1 -run 'TestConformanceRequirements/DET-' -v -timeout=10m

gate-build: gate-determinism
	@echo "=== GATE: BUILD ==="
	CGO_ENABLED=0 go build -trimpath -buildvcs=false -ldflags="-s -w -buildid=" -o jcs-canon ./cmd/jcs-canon
	@sha256sum jcs-canon
	@echo "PASS: static binary built"

gate-all: gate-build
	@echo "=== ALL GATES PASSED ==="
