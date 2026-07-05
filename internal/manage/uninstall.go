package manage

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Uninstall stops the service and removes installed files. Config/data are
// preserved unless Purge is true or the user answers no to the keep prompt.
func Uninstall(opts Options) error {
	opts = opts.withDefaults()
	if err := requireLinuxRoot(opts.Root); err != nil {
		return err
	}
	if err := runSystemctlAllowMissingUnit(opts, "disable", "--now", ServiceName); err != nil {
		return err
	}
	if err := removeIfExists(opts.rooted("/etc/systemd/system/sb-fox.service")); err != nil {
		return err
	}
	if err := runSystemctl(opts, "daemon-reload"); err != nil {
		return err
	}
	if err := runSystemctlAllowMissingUnit(opts, "reset-failed", ServiceName); err != nil {
		return err
	}
	binaryPath, err := resolveBinaryPath(opts.BinaryPath)
	if err != nil {
		return err
	}
	if err := removeIfExists(binaryPath); err != nil {
		return err
	}
	purge := opts.Purge
	if !purge {
		keep, err := askKeepData(opts)
		if err != nil {
			return err
		}
		purge = !keep
	}
	if purge {
		if err := os.RemoveAll(opts.rooted("/etc/sb-fox")); err != nil {
			return fmt.Errorf("remove config directory: %w", err)
		}
		if err := os.RemoveAll(opts.rooted(opts.DataDir)); err != nil {
			return fmt.Errorf("remove data directory: %w", err)
		}
		if err := removeIfExists(opts.rooted(DefaultSocketPath)); err != nil {
			return err
		}
		fmt.Fprintln(opts.Stdout, "sb-fox uninstalled and data removed")
		return nil
	}
	fmt.Fprintln(opts.Stdout, "sb-fox uninstalled; config and data preserved")
	return nil
}

func askKeepData(opts Options) (bool, error) {
	fmt.Fprint(opts.Stdout, "保留配置和数据? [Y/n]: ")
	reader := bufio.NewReader(opts.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "" || answer == "y" || answer == "yes", nil
}
