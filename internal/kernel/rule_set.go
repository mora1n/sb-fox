package kernel

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FormatRuleSet validates and formats sing-box source-format rule-set JSON.
func (k *Kernel) FormatRuleSet(source []byte) ([]byte, error) {
	return k.runRuleSet(source, "input.json", "", "rule-set", "format")
}

// DecompileRuleSet converts binary SRS input to source-format JSON.
func (k *Kernel) DecompileRuleSet(binary []byte) ([]byte, error) {
	return k.runRuleSet(binary, "input.srs", "output.json", "rule-set", "decompile")
}

// CompileRuleSet converts source-format JSON input to binary SRS.
func (k *Kernel) CompileRuleSet(source []byte) ([]byte, error) {
	return k.runRuleSet(source, "input.json", "output.srs", "rule-set", "compile")
}

func (k *Kernel) runRuleSet(input []byte, inputName, outputName string, args ...string) ([]byte, error) {
	if !k.Available() {
		return nil, fmt.Errorf("kernel: binary unavailable")
	}
	dir := k.DataDir
	if dir == "" {
		dir = os.TempDir()
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("kernel: create data directory: %w", err)
	}
	workDir, err := os.MkdirTemp(dir, "sb-fox-ruleset-*")
	if err != nil {
		return nil, fmt.Errorf("kernel: create rule-set workspace: %w", err)
	}
	defer os.RemoveAll(workDir)
	if err := os.Chmod(workDir, 0o700); err != nil {
		return nil, fmt.Errorf("kernel: protect rule-set workspace: %w", err)
	}
	inputPath := filepath.Join(workDir, inputName)
	if err := os.WriteFile(inputPath, input, 0o600); err != nil {
		return nil, fmt.Errorf("kernel: write rule-set input: %w", err)
	}
	commandArgs := append(append([]string{}, args...), inputPath)
	outputPath := ""
	if outputName != "" {
		outputPath = filepath.Join(workDir, outputName)
		commandArgs = append(commandArgs, "--output", outputPath)
	}
	ctx, cancel := context.WithTimeout(context.Background(), k.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, k.Path, commandArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if ctx.Err() != nil {
			message = ctx.Err().Error()
		} else if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("kernel: %s: %s", strings.Join(args, " "), message)
	}
	if outputPath == "" {
		return stdout.Bytes(), nil
	}
	output, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("kernel: read rule-set output: %w", err)
	}
	return output, nil
}
