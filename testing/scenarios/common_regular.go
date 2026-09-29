//go:build !coverage

package scenarios

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

func BuildXray() error {
	genTestBinaryPath()
	if _, err := os.Stat(testBinaryPath); err == nil {
		return nil
	}

	fmt.Printf("Building Xray into path (%s)\n", testBinaryPath)
	cmd := exec.CommandContext(context.Background(), "go", "build", "-o="+testBinaryPath, GetSourcePath()) //nolint:gosec // constant command; the only variable is the repo path this test lives in
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func RunXrayProtobuf(config []byte) *exec.Cmd {
	genTestBinaryPath()
	proc := exec.CommandContext(context.Background(), testBinaryPath, "-config=stdin:", "-format=pb")
	proc.Stdin = bytes.NewBuffer(config)
	proc.Stderr = os.Stderr
	proc.Stdout = os.Stdout

	return proc
}
