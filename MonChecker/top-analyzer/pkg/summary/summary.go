package summary

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/parth2601/monchecker/MonChecker/top-analyzer/pkg/parser"
	"github.com/parth2601/monchecker/MonChecker/top-analyzer/pkg/power"
	"github.com/parth2601/monchecker/MonChecker/top-analyzer/pkg/temperature"
)

type SystemSummary struct {
	Timestamp     time.Time `json:"timestamp"`
	LastCrashFile string    `json:"last_crash_file,omitempty"`
	LastCrashTime time.Time `json:"last_crash_time,omitempty"`
	CPU           struct {
		User   float64 `json:"user"`
		System float64 `json:"system"`
		Idle   float64 `json:"idle"`
		Load1  float64 `json:"load1"`
		Load5  float64 `json:"load5"`
		Load15 float64 `json:"load15"`
	} `json:"cpu"`
	Memory struct {
		Total  uint64  `json:"total"`
		Used   uint64  `json:"used"`
		Free   uint64  `json:"free"`
		UsedPc float64 `json:"used_percent"`
	} `json:"memory"`
	Temperature struct {
		Sensors map[string]struct {
			Value    float64 `json:"value"`
			Location string  `json:"location"`
			MaxTemp  float64 `json:"max_temp"`
			AvgTemp  float64 `json:"avg_temp"`
		} `json:"sensors"`
		MaxTemp float64              `json:"max_temp"`
		AvgTemp float64              `json:"avg_temp"`
		History map[string][]float64 `json:"history"`
	} `json:"temperature"`
	Filesystem struct {
		Partitions map[string]struct {
			Device     string  `json:"device"`
			Size       int64   `json:"size"`
			Used       int64   `json:"used"`
			Available  int64   `json:"available"`
			UsedPct    float64 `json:"used_percent"`
			FreeSpace  float64 `json:"free_space_percent"`
			MountPoint string  `json:"mount_point"`
			Critical   bool    `json:"critical"`
		} `json:"partitions"`
		History map[string][]float64 `json:"history"` // History of free space percentage
	} `json:"filesystem"`
	Processes struct {
		Total        int `json:"total"`
		Running      int `json:"running"`
		Sleeping     int `json:"sleeping"`
		Uninterr     int `json:"uninterruptible"`
		Zombie       int `json:"zombie"`
		HighCPU      int `json:"high_cpu"`
		HighMem      int `json:"high_memory"`
		HighCPUProcs []struct {
			Name       string  `json:"name"`
			CPUPercent float64 `json:"cpu_percent"`
		} `json:"high_cpu_processes"`
	} `json:"processes"`
	SystemStress float64 `json:"system_stress"`
}

func New() *SystemSummary {
	return &SystemSummary{
		Temperature: struct {
			Sensors map[string]struct {
				Value    float64 `json:"value"`
				Location string  `json:"location"`
				MaxTemp  float64 `json:"max_temp"`
				AvgTemp  float64 `json:"avg_temp"`
			} `json:"sensors"`
			MaxTemp float64              `json:"max_temp"`
			AvgTemp float64              `json:"avg_temp"`
			History map[string][]float64 `json:"history"`
		}{
			Sensors: make(map[string]struct {
				Value    float64 `json:"value"`
				Location string  `json:"location"`
				MaxTemp  float64 `json:"max_temp"`
				AvgTemp  float64 `json:"avg_temp"`
			}),
			History: make(map[string][]float64),
		},
		Filesystem: struct {
			Partitions map[string]struct {
				Device     string  `json:"device"`
				Size       int64   `json:"size"`
				Used       int64   `json:"used"`
				Available  int64   `json:"available"`
				UsedPct    float64 `json:"used_percent"`
				FreeSpace  float64 `json:"free_space_percent"`
				MountPoint string  `json:"mount_point"`
				Critical   bool    `json:"critical"`
			} `json:"partitions"`
			History map[string][]float64 `json:"history"`
		}{
			Partitions: make(map[string]struct {
				Device     string  `json:"device"`
				Size       int64   `json:"size"`
				Used       int64   `json:"used"`
				Available  int64   `json:"available"`
				UsedPct    float64 `json:"used_percent"`
				FreeSpace  float64 `json:"free_space_percent"`
				MountPoint string  `json:"mount_point"`
				Critical   bool    `json:"critical"`
			}),
			History: make(map[string][]float64),
		},
	}
}

func (s *SystemSummary) Update(stats *parser.SystemStats, powerStats *power.PowerStats, tempStats *temperature.TemperatureStats, crashFile string) {
	s.Timestamp = time.Now()
	if crashFile != "" {
		s.LastCrashFile = crashFile
		s.LastCrashTime = time.Now()
	}

	// Update CPU stats
	s.CPU.User = stats.CPU.User
	s.CPU.System = stats.CPU.Sys
	s.CPU.Idle = stats.CPU.Idle
	s.CPU.Load1 = stats.LoadAverage.One
	s.CPU.Load5 = stats.LoadAverage.Five
	s.CPU.Load15 = stats.LoadAverage.Fifteen

	// Update memory stats
	total := stats.Memory.Total
	s.Memory.Total = uint64(total)
	s.Memory.Used = uint64(stats.Memory.Used)
	s.Memory.Free = uint64(stats.Memory.Free)
	s.Memory.UsedPc = float64(s.Memory.Used) / float64(total) * 100

	// Update temperature stats
	s.Temperature.Sensors = make(map[string]struct {
		Value    float64 `json:"value"`
		Location string  `json:"location"`
		MaxTemp  float64 `json:"max_temp"`
		AvgTemp  float64 `json:"avg_temp"`
	})

	// Add all detected sensors with their locations and update history
	for sensorName, temp := range tempStats.Sensors {
		location := "Unknown"
		if sensorName == "f10e4078.thermal" {
			location = "CPU Core"
		} else if sensorName == "lm75" {
			location = "External I2C Sensor"
		}

		s.Temperature.Sensors[sensorName] = struct {
			Value    float64 `json:"value"`
			Location string  `json:"location"`
			MaxTemp  float64 `json:"max_temp"`
			AvgTemp  float64 `json:"avg_temp"`
		}{
			Value:    temp,
			Location: location,
			MaxTemp:  temp, // Initial value, will be updated
			AvgTemp:  temp, // Initial value, will be updated
		}

		s.Temperature.History[sensorName] = append(s.Temperature.History[sensorName], temp)
		if len(s.Temperature.History[sensorName]) > 10 {
			s.Temperature.History[sensorName] = s.Temperature.History[sensorName][1:]
		}
	}

	// Calculate max and avg for each sensor and overall
	s.Temperature.MaxTemp = -100.0 // Initialize to very low value
	overallSum := 0.0
	overallCount := 0

	for sensorName, temps := range s.Temperature.History {
		sensorMax := -100.0
		sensorSum := 0.0

		for _, temp := range temps {
			if temp > sensorMax {
				sensorMax = temp
			}
			sensorSum += temp

			// Update overall max
			if temp > s.Temperature.MaxTemp {
				s.Temperature.MaxTemp = temp
			}

			overallSum += temp
			overallCount++
		}

		// Update sensor max and avg
		sensor := s.Temperature.Sensors[sensorName]
		sensor.MaxTemp = sensorMax
		sensor.AvgTemp = sensorSum / float64(len(temps))
		s.Temperature.Sensors[sensorName] = sensor
	}

	if overallCount > 0 {
		s.Temperature.AvgTemp = overallSum / float64(overallCount)
	}

	// Update process stats
	s.Processes.Total = len(stats.Processes)
	stateCount := make(map[string]int)
	highCPU := 0
	highMem := 0
	s.Processes.HighCPUProcs = make([]struct {
		Name       string  `json:"name"`
		CPUPercent float64 `json:"cpu_percent"`
	}, 0)

	for _, proc := range stats.Processes {
		stateCount[proc.State]++
		if proc.CPUPercent > 10 {
			highCPU++
			s.Processes.HighCPUProcs = append(s.Processes.HighCPUProcs, struct {
				Name       string  `json:"name"`
				CPUPercent float64 `json:"cpu_percent"`
			}{
				Name:       proc.Command,
				CPUPercent: proc.CPUPercent,
			})
		}
		if proc.VSZPercent > 5 {
			highMem++
		}
	}

	s.Processes.Running = stateCount["R"]
	s.Processes.Sleeping = stateCount["S"]
	s.Processes.Uninterr = stateCount["D"]
	s.Processes.Zombie = stateCount["Z"]
	s.Processes.HighCPU = highCPU
	s.Processes.HighMem = highMem

	// Calculate system stress
	s.SystemStress = calculateSystemStress(s)

	// Update filesystem stats
	if stats.Filesystem != nil {
		// Initialize if not already initialized
		if s.Filesystem.Partitions == nil {
			s.Filesystem.Partitions = make(map[string]struct {
				Device     string  `json:"device"`
				Size       int64   `json:"size"`
				Used       int64   `json:"used"`
				Available  int64   `json:"available"`
				UsedPct    float64 `json:"used_percent"`
				FreeSpace  float64 `json:"free_space_percent"`
				MountPoint string  `json:"mount_point"`
				Critical   bool    `json:"critical"`
			})
		}
		if s.Filesystem.History == nil {
			s.Filesystem.History = make(map[string][]float64)
		}

		// Update partitions info
		for mount, fs := range stats.Filesystem {
			freeSpace := 100.0 - fs.UsedPct
			s.Filesystem.Partitions[mount] = struct {
				Device     string  `json:"device"`
				Size       int64   `json:"size"`
				Used       int64   `json:"used"`
				Available  int64   `json:"available"`
				UsedPct    float64 `json:"used_percent"`
				FreeSpace  float64 `json:"free_space_percent"`
				MountPoint string  `json:"mount_point"`
				Critical   bool    `json:"critical"`
			}{
				Device:     fs.Device,
				Size:       fs.Size,
				Used:       fs.Used,
				Available:  fs.Available,
				UsedPct:    fs.UsedPct,
				FreeSpace:  freeSpace,
				MountPoint: fs.MountPoint,
				Critical:   fs.Critical,
			}

			// Update history
			s.Filesystem.History[mount] = append(s.Filesystem.History[mount], freeSpace)
			if len(s.Filesystem.History[mount]) > 10 {
				s.Filesystem.History[mount] = s.Filesystem.History[mount][1:]
			}
		}
	}
}

func calculateSystemStress(s *SystemSummary) float64 {
	stress := 0.0

	// CPU stress factors (total CPU usage)
	totalCPU := s.CPU.User + s.CPU.System
	if totalCPU > 90 {
		stress += 30
	} else if totalCPU > 70 {
		stress += 20
	} else if totalCPU > 50 {
		stress += 10
	}

	// Memory stress factors
	if s.Memory.UsedPc > 90 {
		stress += 30
	} else if s.Memory.UsedPc > 70 {
		stress += 20
	} else if s.Memory.UsedPc > 50 {
		stress += 10
	}

	// Load average stress
	load := s.CPU.Load1
	if load > 10 {
		stress += 30
	} else if load > 5 {
		stress += 20
	} else if load > 2 {
		stress += 10
	}

	// Temperature stress - Operating range: -25°C to 75°C
	if s.Temperature.MaxTemp > 70 {
		// Approaching the upper limit of operating range
		stress += 30
	} else if s.Temperature.MaxTemp > 60 {
		stress += 20
	} else if s.Temperature.MaxTemp > 50 {
		stress += 10
	} else if s.Temperature.MaxTemp < -20 {
		// Approaching the lower limit of operating range
		stress += 20
	} else if s.Temperature.MaxTemp < -10 {
		stress += 10
	}

	// Process stress factors
	if s.Processes.Uninterr > 5 {
		stress += 20
	}
	if s.Processes.HighCPU > 10 {
		stress += 20
	}

	// Filesystem stress factors
	for mount, partition := range s.Filesystem.Partitions {
		// Critical low space on any partition
		if partition.FreeSpace < 10 {
			// Higher stress for critical system partitions
			if mount == "/" {
				stress += 40 // Root partition critical
			} else if mount == "/boot" {
				stress += 30 // Boot partition critical
			} else {
				stress += 20 // Other partition critical
			}
		} else if partition.FreeSpace < 20 {
			// Warning level (less than 20% free)
			if mount == "/" {
				stress += 20 // Root partition low
			} else if mount == "/boot" {
				stress += 15 // Boot partition low
			} else {
				stress += 10 // Other partition low
			}
		}
	}

	return min(stress, 100.0)
}

func (s *SystemSummary) Save(filename string) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal summary: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write summary: %w", err)
	}

	return nil
}

// helper function
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
