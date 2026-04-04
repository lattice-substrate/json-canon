package main

import (
	"fmt"
	"io"

	"github.com/lattice-substrate/json-canon/offline/replay"
)

var bindOfficialES6ProofFunc = bindOfficialES6Proof

type officialES6Proof struct {
	Lines  int
	SHA256 string
}

func bindOfficialES6Proof(outputDir string, stdout io.Writer, evidencePaths ...string) (officialES6Proof, error) {
	if err := runOfficialES6100MGate(outputDir, stdout); err != nil {
		return officialES6Proof{}, err
	}
	proof := officialES6Proof{
		Lines:  replay.OfficialES6CorpusFullLines,
		SHA256: replay.OfficialES6CorpusFullSHA256,
	}
	for _, evidencePath := range evidencePaths {
		if evidencePath == "" {
			continue
		}
		if err := writeOfficialES6ProofToEvidence(evidencePath, proof); err != nil {
			return officialES6Proof{}, err
		}
	}
	return proof, nil
}

func writeOfficialES6ProofToEvidence(path string, proof officialES6Proof) error {
	evidence, err := replay.LoadEvidence(path)
	if err != nil {
		return fmt.Errorf("load evidence %s: %w", path, err)
	}
	evidence.OfficialES6CorpusLines = proof.Lines
	evidence.OfficialES6CorpusSHA256 = proof.SHA256
	if err := writeEvidenceBundleFunc(path, evidence); err != nil {
		return fmt.Errorf("write evidence %s: %w", path, err)
	}
	return nil
}
