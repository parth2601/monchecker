package filesystem

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// FilesystemStats represents statistics about a filesystem
type FilesystemStats struct {
	Filesystems map[string]Filesystem
}

// Filesystem represents a single filesystem
type Filesystem struct {
	Device     string
	Size       int64 // in bytes
	Used       int64 // in bytes
	Available  int64 // in bytes
	UsedPct    float64
	MountPoint string
	Critical   bool // when free space < 10%
}

// ReadFilesystemStats reads filesystem statistics using df command
func ReadFilesystemStats() (*FilesystemStats, error) {
	cmd := exec.Command("df", "-B1") // Get sizes in bytes for precision
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to execute df command: %w", err)
	}

	return parseFilesystemStats(string(output))
}

// parseFilesystemStats parses the output of df command
func parseFilesystemStats(output string) (*FilesystemStats, error) {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return nil, fmt.Errorf("invalid df output format")
	}

	stats := &FilesystemStats{
		Filesystems: make(map[string]Filesystem),
	}

	// Skip header line
	for _, line := range lines[1:] {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle multi-line entries (specifically for lines with newlines)
		if !strings.HasPrefix(line, "/") && !strings.HasPrefix(line, "tmpfs") &&
			!strings.HasPrefix(line, "devtmpfs") && !strings.HasPrefix(line, "none") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 6 {
			continue
		}

		device := fields[0]
		mountPoint := fields[5]

		// Filter out filesystems we care about (main storage partitions)
		if !isMainPartition(mountPoint) {
			continue
		}

		size, _ := strconv.ParseInt(fields[1], 10, 64)
		used, _ := strconv.ParseInt(fields[2], 10, 64)
		available, _ := strconv.ParseInt(fields[3], 10, 64)

		// Parse percentage field, remove the '%' character
		usedPctStr := strings.TrimSuffix(fields[4], "%")
		usedPct, _ := strconv.ParseFloat(usedPctStr, 64)

		// Create filesystem entry
		fs := Filesystem{
			Device:     device,
			Size:       size,
			Used:       used,
			Available:  available,
			UsedPct:    usedPct,
			MountPoint: mountPoint,
			Critical:   usedPct > 90, // Critical when used space > 90% (free space < 10%)
		}

		stats.Filesystems[mountPoint] = fs
	}

	return stats, nil
}

// isMainPartition checks if the mount point is one of the main partitions we want to monitor
func isMainPartition(mountPoint string) bool {
	mainPartitions := []string{
		"/",           // root partition
		"/boot",       // boot partition
		"/mnt/user",   // user partition
		"/mnt/config", // config partition
	}

	for _, p := range mainPartitions {
		if mountPoint == p {
			return true
		}
	}
	return false
}

// String returns a string representation of filesystem stats
func (fs *FilesystemStats) String() string {
	var sb strings.Builder
	for mount, stats := range fs.Filesystems {
		free := float64(stats.Available) / float64(stats.Size) * 100.0
		status := "OK"
		if stats.Critical {
			status = "CRITICAL"
		} else if free < 20 {
			status = "WARNING"
		}

		sb.WriteString(fmt.Sprintf("%s (%s): %.1f%% used, %.2f GB free [%s]\n",
			mount,
			stats.Device,
			stats.UsedPct,
			float64(stats.Available)/1024.0/1024.0/1024.0,
			status))
	}
	return sb.String()
}
