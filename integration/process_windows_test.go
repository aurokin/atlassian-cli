//go:build integration && windows

package integration

import "os/exec"

// Live product commands do not spawn children. CommandContext kills the binary;
// WaitDelay bounds inherited output handles even if a child outlives it.
func boundProcess(cmd *exec.Cmd) {}
