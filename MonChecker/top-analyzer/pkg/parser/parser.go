package parser

import (
	"bufio"
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/monchecker/top-analyzer/pkg/temperature"
)

// SystemStats represents the system statistics
type SystemStats struct {
	Timestamp   time.Time
	CPU         CPU
	Memory      Memory
	LoadAverage LoadAverage
	Processes   []Process
	Temperature temperature.TemperatureStats
	Filesystem  map[string]FilesystemStats
}

// CPU represents CPU statistics
type CPU struct {
	User float64
	Sys  float64
	Nice float64
	Idle float64
	IO   float64
	IRQ  float64
	SIRQ float64
}

// Memory represents memory statistics
type Memory struct {
	Total   int64
	Used    int64
	Free    int64
	Shared  int64
	Buffers int64
	Cached  int64
}

// LoadAverage represents load averages for 1, 5, and 15 minutes
type LoadAverage struct {
	One     float64
	Five    float64
	Fifteen float64
}

// Process represents a process
type Process struct {
	PID        int
	PPID       int
	User       string
	Priority   int
	Nice       int
	VSZ        int64
	VSZPercent float64
	RSS        int64
	State      string
	CPU        int
	CPUPercent float64
	MemPercent float64
	Time       string
	Command    string
}

// FilesystemStats represents statistics for a filesystem
type FilesystemStats struct {
	Device     string
	Size       int64
	Used       int64
	Available  int64
	UsedPct    float64
	MountPoint string
	Critical   bool
}

func ParseTopOutput(output []byte) (*SystemStats, error) {
	stats := &SystemStats{}
	scanner := bufio.NewScanner(bytes.NewReader(output))
	lines := []string{}
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 {
		return stats, nil
	}

	// Detect format
	isBusyBox := false
	isGNUTop := false
	for _, l := range lines {
		if strings.HasPrefix(l, "Mem:") && strings.Contains(l, "used") && strings.Contains(l, "free") {
			isBusyBox = true
		}
		if strings.HasPrefix(l, "%Cpu(s):") || strings.HasPrefix(l, "MiB Mem :") {
			isGNUTop = true
		}
	}

	if isBusyBox {
		parseBusyBoxTop(lines, stats)
	} else if isGNUTop {
		parseGNUTop(lines, stats)
	} else {
		return nil, fmt.Errorf("unknown top output format")
	}
	return stats, nil
}

func parseBusyBoxTop(lines []string, stats *SystemStats) {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "Mem:") {
			parts := strings.Fields(line)
			if len(parts) >= 10 {
				stats.Memory.Used = parseKValue(parts[1])
				stats.Memory.Free = parseKValue(parts[3])
				stats.Memory.Shared = parseKValue(parts[5])
				stats.Memory.Buffers = parseKValue(parts[7])
				stats.Memory.Cached = parseKValue(parts[9])
				stats.Memory.Total = stats.Memory.Used + stats.Memory.Free + stats.Memory.Shared + stats.Memory.Buffers + stats.Memory.Cached
			}
		}
		if strings.HasPrefix(line, "CPU:") {
			parts := strings.Fields(line)
			if len(parts) >= 14 {
				stats.CPU.User = parsePercent(parts[1])
				stats.CPU.Sys = parsePercent(parts[3])
				stats.CPU.Nice = parsePercent(parts[5])
				stats.CPU.Idle = parsePercent(parts[7])
				stats.CPU.IO = parsePercent(parts[9])
				stats.CPU.IRQ = parsePercent(parts[11])
				stats.CPU.SIRQ = parsePercent(parts[13])
			}
		}
		if strings.HasPrefix(line, "Load average:") {
			parts := strings.Fields(line)
			if len(parts) >= 5 {
				stats.LoadAverage.One = parseFloat(parts[2])
				stats.LoadAverage.Five = parseFloat(parts[3])
				stats.LoadAverage.Fifteen = parseFloat(parts[4])
			}
		}
		if strings.HasPrefix(line, "  PID") {
			// Process table header
			for j := i + 1; j < len(lines); j++ {
				parts := strings.Fields(lines[j])
				if len(parts) >= 8 {
					proc := Process{}
					proc.PID = parseInt(parts[0])
					proc.PPID = parseInt(parts[1])
					proc.User = parts[2]
					proc.State = parts[3]
					proc.VSZ = parseKValue(parts[4])
					proc.VSZPercent = parsePercent(parts[5])
					proc.CPU = parseInt(parts[6])
					proc.CPUPercent = parsePercent(parts[7])
					proc.Command = strings.Join(parts[8:], " ")
					stats.Processes = append(stats.Processes, proc)
				}
			}
			break
		}
	}
}

func parseGNUTop(lines []string, stats *SystemStats) {
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "%Cpu(s):") {
			// Example: %Cpu(s):  0.0 us,  0.0 sy,  0.0 ni,100.0 id,  0.0 wa,  0.0 hi,  0.0 si,  0.0 st
			cpuFields := strings.Split(line, ":")[1]
			cpuParts := strings.Split(cpuFields, ",")
			for _, part := range cpuParts {
				fields := strings.Fields(strings.TrimSpace(part))
				if len(fields) == 2 {
					val := parseFloat(fields[0])
					switch fields[1] {
					case "us":
						stats.CPU.User = val
					case "sy":
						stats.CPU.Sys = val
					case "ni":
						stats.CPU.Nice = val
					case "id":
						stats.CPU.Idle = val
					case "wa":
						stats.CPU.IO = val
					case "hi":
						stats.CPU.IRQ = val
					case "si":
						stats.CPU.SIRQ = val
					}
				}
			}
		}
		if strings.HasPrefix(line, "MiB Mem :") {
			// Example: MiB Mem :   2017.4 total,    348.5 free,    447.2 used,   1284.3 buff/cache
			memFields := strings.Split(line, ":")[1]
			memParts := strings.Split(memFields, ",")
			for _, part := range memParts {
				fields := strings.Fields(strings.TrimSpace(part))
				if len(fields) == 2 {
					val := parseFloat(fields[0])
					switch fields[1] {
					case "total":
						stats.Memory.Total = int64(val * 1024 * 1024)
					case "free":
						stats.Memory.Free = int64(val * 1024 * 1024)
					case "used":
						stats.Memory.Used = int64(val * 1024 * 1024)
					case "buff/cache":
						stats.Memory.Cached = int64(val * 1024 * 1024)
					}
				}
			}
		}
		if strings.Contains(line, "load average:") {
			// Example: top - 07:32:26 up 4 days, 23:37,  1 user,  load average: 0.10, 0.24, 0.20
			idx := strings.Index(line, "load average:")
			if idx != -1 {
				loads := strings.Split(line[idx+len("load average:"):], ",")
				if len(loads) >= 3 {
					stats.LoadAverage.One = parseFloat(strings.TrimSpace(loads[0]))
					stats.LoadAverage.Five = parseFloat(strings.TrimSpace(loads[1]))
					stats.LoadAverage.Fifteen = parseFloat(strings.TrimSpace(loads[2]))
				}
			}
		}
		if strings.HasPrefix(line, "  PID") || strings.HasPrefix(line, "PID ") {
			// Process table header
			for j := i + 1; j < len(lines); j++ {
				parts := strings.Fields(lines[j])
				if len(parts) >= 12 {
					proc := Process{}
					proc.PID = parseInt(parts[0])
					proc.User = parts[1]
					proc.Priority = parseInt(parts[2])
					proc.Nice = parseInt(parts[3])
					proc.VSZ = int64(parseInt(parts[4]))
					proc.RSS = int64(parseInt(parts[5]))
					proc.State = parts[6]
					proc.CPUPercent = parseFloat(parts[7])
					proc.MemPercent = parseFloat(parts[8])
					proc.Time = parts[9]
					proc.Command = strings.Join(parts[10:], " ")
					stats.Processes = append(stats.Processes, proc)
				}
			}
			break
		}
	}
}

func parseKValue(s string) int64 {
	s = strings.TrimSuffix(s, "K")
	val, _ := strconv.ParseInt(s, 10, 64)
	return val * 1024
}

func parsePercent(s string) float64 {
	s = strings.TrimSuffix(s, "%")
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func parseFloat(s string) float64 {
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

func parseInt(s string) int {
	val, _ := strconv.Atoi(s)
	return val
}
