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
)

func echo(text, file string) error {
	f, err := os.OpenFile(file, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("failed to update cgroup file %q: %w", file, err)
	}

	defer func() { _ = f.Close() }()

	if _, err := f.WriteString(text); err != nil {
		return fmt.Errorf("failed to update cgroup file %q: %w", file, err)
	}

	return nil
}

func cgroupName(jid ID) string {
	return "job-" + string(jid)
}

// SetupCgroup creates a new v2 cgroup for job jid, applying limits
func (j *Job) setupCgroup() (string, error) {
	createdCG := false
	if j.cgroup == "" {
		ok, cgctrl := FindCroupMount()
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

	if j.limits.MaxDiskIOBytes > 0 {
		blocks, err := ListBlockDevs()
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

	if j.limits.MaxRamBytes > 0 {
		err := echo(itoa(j.limits.MaxRamBytes), filepath.Join(j.cgroup, "memory.max"))
		if err != nil {
			if createdCG {
				_ = os.Remove(j.cgroup)
			}
			return "", fmt.Errorf("failed to configure RAM limits: %w", err)
		}
	}

	if j.limits.CPU > 0. {
		period := float32(10000.)
		txt := fmt.Sprintf("%f.2 %f", period*j.limits.CPU, period)
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

func AddPidToCgroup(pid int, cgPath string) error {
	if err := echo(strconv.Itoa(pid), filepath.Join(cgPath, "cgroup.procs")); err != nil {
		return fmt.Errorf("failed to add the pid to the new group: %w", err)
	}
	return nil
}

func ListBlockDevs() ([]string, error) {
	var rt []string

	cmd := exec.Command("lsblk", "-d")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("failed to enlist block devices: %w", err)
	}
	s := bufio.NewScanner(bytes.NewReader(out))
	for s.Scan() {
		parts := strings.Fields(s.Text())
		if parts[5] == "disk" {
			rt = append(rt, parts[1])
		}
	}

	if s.Err() != nil {
		return nil, fmt.Errorf("failed to scan block devices: %w", s.Err())
	}

	return rt, nil
}

func FindCroupMount() (bool, string) {
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

func SetupProc(cgPath string, identity ExecIdentity) error {
	if err := RemountProc(); err != nil {
		return err
	}

	if err := AddPidToCgroup(os.Getpid(), cgPath); err != nil {
		return err
	}

	if err := SetupIDs(identity); err != nil {
		return err
	}

	return nil
}

// SetupIDs sets UID and GID of the current process
func SetupIDs(ids ExecIdentity) error {
	prevGid := os.Getuid()
	if err := syscall.Setgid(ids.Gid); err != nil {
		return err
	}

	if err := syscall.Setuid(ids.Uid); err != nil {
		syscall.Setgid(prevGid)
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
