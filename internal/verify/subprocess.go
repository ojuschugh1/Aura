package verify

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ojuschugh1/aura/internal/subprocess"
)

// RunClaimcheck invokes the external claimcheck binary with the given transcript
// path and parses its JSON output into a VerifyResult.
// If the binary is not found, a descriptive error is returned (graceful degradation).
func RunClaimcheck(transcriptPath string) (*VerifyResult, error) {
	binPath, err := subprocess.ResolveBinary("claimcheck")
	if err != nil {
		return nil, fmt.Errorf(
			"claimcheck not available — install it or run `aura init --install-deps`: %w", err)
	}

	runner := &subprocess.Runner{BinaryPath: binPath} // uses DefaultTimeout
	res, err := runner.Run(context.Background(), []string{transcriptPath}, nil)
	if err != nil {
		return nil, fmt.Errorf("claimcheck execution failed: %w", err)
	}

	var result VerifyResult
	if err := json.Unmarshal(res.Stdout, &result); err != nil {
		return nil, fmt.Errorf("parse claimcheck output: %w", err)
	}
	return &result, nil
}
