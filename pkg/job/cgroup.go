//+go:build linux

package job

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/rs/zerolog/log"
)

// echo appends string "text" to file "file".
// It's intended to use with cgroup fs files, which usually exit
// for testing we're using in-mem pseudo-cgroup FS, for this case we make a second shot
// and open the file with O_CREATE
func echo(text, file string) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		f, err = os.OpenFile(file, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
	}
	if err != nil {
		return fmt.Errorf("failed to update cgroup file %q: %w", file, err)
	}

	defer func() {
		if err := f.Close(); err != nil {
			log.Warn().Err(err).Str("file", file).Msg("failed to close the file")
		}
	}()

	if _, err := f.WriteString(text + "\n"); err != nil {
		return fmt.Errorf("failed to update cgroup file %q: %w", file, err)
	}

	return nil
}

// cgroupName creates a cgroup name based on Job ID
func cgroupName(jid ID) string {
	return "job-" + string(jid)
}

// SetupCgroup creates a new v2 cgroup for job jid, applying limits.
func (j *Job) setupCgroup() (string, error) {
	createdCG := false
	if j.cgroup == "" {
		ok, cgctrl := findCgroupMount()
		if !ok {
			return "", fmt.Errorf("cgroup2 controller is not mounted")
		}

		newCgPath := filepath.Join(cgctrl, cgroupName(j.ID))
		err := os.MkdirAll(newCgPath, 0600)
		if err != nil {
			return "", fmt.Errorf("failed to create cgroup: %w", err)
		}
		j.cgroup = newCgPath
		createdCG = true
	}

	// limit disk IO
	if j.limits.MaxDiskIOBytes > 0 {
		blocks, err := listBlockDevs()
		if err != nil {
			if createdCG {
				_ = os.Remove(j.cgroup)
			}
			return "", fmt.Errorf("failed to configure IO limits: %w", err)
		}

		rate := itoa(j.limits.MaxDiskIOBytes)

		for _, b := range blocks {
			txt := fmt.Sprintf("%s rbps=%s wbps=%s", b, rate, rate)
			if err := echo(txt, filepath.Join(j.cgroup, "io.max")); err != nil {
				if createdCG {
					_ = os.Remove(j.cgroup)
				}
				return "", fmt.Errorf("failed to configure IO limits: %w", err)
			}
		}
	}

	// limit RAM
	if j.limits.MaxRamBytes > 0 {
		err := echo(itoa(j.limits.MaxRamBytes), filepath.Join(j.cgroup, "memory.max"))
		if err != nil {
			if createdCG {
				_ = os.Remove(j.cgroup)
			}
			return "", fmt.Errorf("failed to configure RAM limits: %w", err)
		}
	}

	// limit CPU usage
	if j.limits.CPU > 0. {
		period := float32(10000.)
		txt := fmt.Sprintf("%.4f %.4f", period*j.limits.CPU, period)
		err := echo(txt, filepath.Join(j.cgroup, "cpu.max"))
		if err != nil {
			if createdCG {
				_ = os.Remove(j.cgroup)
			}
			return "", fmt.Errorf("failed to configure CPU limits: %w", err)
		}
	}

	return j.cgroup, nil
}

// addPidToCgroup add process pid to cgroup controlled by cgPath
func addPidToCgroup(pid int, cgPath string) error {
	if err := echo(strconv.Itoa(pid), filepath.Join(cgPath, "cgroup.procs")); err != nil {
		return fmt.Errorf("failed to add the pid to the new group: %w", err)
	}
	return nil
}

// ListBlockDevs enumerates all disk root devices
// require 'lsblk' to be installed
func listBlockDevs() ([]string, error) {
	var rt []string

	cmd := exec.Command("lsblk", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to enlist block devices: %w", err)
	}
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if len(parts) < 6 {
			return nil, fmt.Errorf("unexpected output of lsblk: %q", s.Text())
		}
		if parts[5] == "disk" {
			rt = append(rt, parts[1])
		}
	}

	if s.Err() != nil {
		return nil, fmt.Errorf("failed to scan block devices: %w", s.Err())
	}

	return rt, nil
}

// findCgroupMount returns the current mount point of cgroup2 FS if exist
func findCgroupMount() (bool, string) {
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false, ""
	}

	defer func() { _ = f.Close() }()

	r := bufio.NewScanner(f)
	for r.Scan() {
		parts := strings.Fields(r.Text())
		if len(parts) < 3 {
			continue
		}
		if parts[0] == "cgroup2" {
			return true, parts[1]
		}
	}

	return false, ""
}

// SetupProc is intended to be called from shim process, adding the process to required cgroup
// and configuring /proc to make tools like top and ps work
func SetupProc(cgPath string, identity ExecIdentity) error {
	if err := remountProc(); err != nil {
		return err
	}

	if err := addPidToCgroup(os.Getpid(), cgPath); err != nil {
		return err
	}

	if err := setupIDs(identity); err != nil {
		return err
	}

	return nil
}

// setupIDs sets UID and GID of the current process.
func setupIDs(ids ExecIdentity) error {
	prevGid := os.Getuid()
	if err := syscall.Setgid(ids.GID); err != nil {
		return err
	}

	if err := syscall.Setuid(ids.UID); err != nil {
		_ = syscall.Setgid(prevGid)
		return err
	}
	return nil
}

// itoa converts int64 value to a string usable in cgroup files
// if x>0 it will be formatted as is;
// otherwise return 'max'
func itoa(x int64) string {
	if x <= 0 {
		return "max"
	}
	return strconv.FormatInt(x, 10)
}
