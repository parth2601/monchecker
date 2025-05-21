package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"os/exec"

	"github.com/parth2601/monchecker/top-analyzer/pkg/filesystem"
	"github.com/parth2601/monchecker/top-analyzer/pkg/parser"
	"github.com/parth2601/monchecker/top-analyzer/pkg/summary"
	"github.com/parth2601/monchecker/top-analyzer/pkg/temperature"
	"github.com/parth2601/monchecker/top-analyzer/pkg/trend"
	"github.com/sirupsen/logrus"
)

var (
	interval         = flag.Duration("interval", 5*time.Second, "Interval between top command executions")
	history          = flag.Int("history", 10, "Number of samples to keep in history")
	logFile          = flag.String("log", "top-analyzer.log", "Path to log file")
	snapshotDir      = flag.String("snapshot-dir", "snapshots", "Directory for snapshots")
	crashDir         = flag.String("crash-dir", "crashes", "Directory for crash dumps")
	summaryDir       = flag.String("summary-dir", "summary", "Directory for summary files")
	snapshotPeriod   = flag.Duration("snapshot-period", 1*time.Hour, "Period between snapshots")
	anomalyThreshold = flag.Float64("anomaly-threshold", 2, "Z-score threshold for anomaly detection (higher = less sensitive)")
	trendThreshold   = flag.Float64("trend-threshold", 0.1, "Trend slope threshold for anomaly detection")
	tempThreshold    = flag.Float64("temp-threshold", 70, "Absolute temperature threshold in °C")
	longTermWindow   = flag.Int("long-term-window", 100, "Number of samples to keep in long-term history")
)

func main() {
	flag.Parse()

	// Create directories
	os.MkdirAll(*snapshotDir, 0755)
	os.MkdirAll(*crashDir, 0755)
	os.MkdirAll(*summaryDir, 0755)

	// Setup logger
	log := logrus.New()
	file, err := os.OpenFile(*logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		log.SetOutput(file)
	}
	log.SetLevel(logrus.InfoLevel)

	// Initialize analyzer with configurable anomaly threshold
	analyzer := trend.NewWithFullOptions(*history, *anomalyThreshold, *trendThreshold, *tempThreshold, *longTermWindow)
	s := summary.New()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Setup crash recovery
	defer func() {
		if r := recover(); r != nil {
			log.Errorf("Panic occurred: %v", r)
			saveCrashDump(analyzer, log)
		}
	}()

	// Create tickers
	statsTicker := time.NewTicker(*interval)
	snapshotTicker := time.NewTicker(*snapshotPeriod)
	lastSummarySave := time.Now()
	defer statsTicker.Stop()
	defer snapshotTicker.Stop()

	// Main monitoring loop
	for {
		select {
		case <-statsTicker.C:
			// Read system stats
			cmd := exec.Command("top", "-b", "-n", "1")
			output, err := cmd.Output()
			if err != nil {
				log.Printf("Failed to run top command: %v", err)
				continue
			}

			// Debug logging for raw top output
			log.Debugf("Raw top output:\n%s", string(output))

			stats, err := parser.ParseTopOutput(output)
			if err != nil {
				log.Printf("Failed to parse top output: %v", err)
				continue
			}

			// Debug logging for CPU and memory stats
			log.Debugf("Raw CPU stats - User: %.1f%%, Sys: %.1f%%, Idle: %.1f%%",
				stats.CPU.User, stats.CPU.Sys, stats.CPU.Idle)

			// Calculate memory percentage safely
			memUsedPct := 0.0
			if stats.Memory.Total > 0 {
				memUsedPct = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
			}
			log.Debugf("Raw Memory stats - Total: %d MB, Used: %d MB, Free: %d MB, Used%%: %.1f%%",
				stats.Memory.Total/1024/1024, stats.Memory.Used/1024/1024, stats.Memory.Free/1024/1024, memUsedPct)

			// Read temperature stats
			tempStats, err := temperature.ReadTemperatureStats()
			if err != nil {
				log.Warnf("Failed to read temperature stats: %v", err)
				// Initialize empty temperature stats structure to avoid null in logs
				tempStats = &temperature.TemperatureStats{
					Sensors: make(map[string]float64),
				}

				// Try to set default values for known sensors for testing
				if os.Getenv("MOCK_TEMP") == "1" {
					tempStats.Sensors["cpu"] = 45.0
					tempStats.Sensors["board"] = 40.0
				}
			}

			// Read filesystem stats
			fsStats, err := filesystem.ReadFilesystemStats()
			if err != nil {
				log.Warnf("Failed to read filesystem stats: %v", err)
				fsStats = &filesystem.FilesystemStats{
					Filesystems: make(map[string]filesystem.Filesystem),
				}
			}

			// Convert filesystem stats to parser format
			stats.Filesystem = make(map[string]parser.FilesystemStats)
			for mountPoint, fs := range fsStats.Filesystems {
				stats.Filesystem[mountPoint] = parser.FilesystemStats{
					Device:     fs.Device,
					Size:       fs.Size,
					Used:       fs.Used,
					Available:  fs.Available,
					UsedPct:    fs.UsedPct,
					MountPoint: fs.MountPoint,
					Critical:   fs.Critical,
				}
			}

			// Debug info to track sensors detected
			if len(tempStats.Sensors) > 0 {
				log.Infof("Temperature sensors detected: %v", tempStats.String())
			} else {
				log.Warnf("No temperature sensors detected")
			}

			// Debug info for filesystem stats
			if len(fsStats.Filesystems) > 0 {
				log.Infof("Filesystem stats: %v", fsStats.String())
			} else {
				log.Warnf("No filesystem stats detected")
			}

			// Update analyzer and summary
			analyzer.AddStats(stats)
			s.Update(stats, nil, tempStats, "")

			// Analyze trends
			trend := analyzer.Analyze()
			if trend != nil {
				// Check for conditions that should trigger a crash dump
				if trend.SystemStress >= 85 ||
					trend.CPUUsage.Anomaly ||
					trend.ProcessCount.Anomaly ||
					trend.Temperature.Anomaly ||
					trend.MemoryUsage.Anomaly ||
					trend.Temperature.ThresholdExceeded ||
					trend.Filesystem.Critical ||
					trend.Filesystem.Anomaly {

					log.Warnf("Detected conditions requiring crash dump:")
					if trend.SystemStress >= 85 {
						log.Warnf("- High system stress: %.1f%%", trend.SystemStress)
					}
					if trend.CPUUsage.Anomaly {
						log.Warnf("- CPU anomaly detected: %.1f%% (threshold: %.1f)", trend.CPUUsage.Mean, trend.CPUUsage.StdDev*(*anomalyThreshold))
					}
					if trend.MemoryUsage.Anomaly {
						log.Warnf("- Memory anomaly detected: %.1f%% (threshold: %.1f)", trend.MemoryUsage.Mean, trend.MemoryUsage.StdDev*(*anomalyThreshold))
					}
					if trend.Temperature.Anomaly {
						log.Warnf("- Temperature anomaly detected: %.1f°C (threshold: %.1f)", trend.Temperature.Mean, trend.Temperature.StdDev*(*anomalyThreshold))
					}
					if trend.Temperature.ThresholdExceeded {
						log.Warnf("- Temperature threshold exceeded: %.1f°C (threshold: %.1f°C)", trend.Temperature.Max, *tempThreshold)
					}
					if trend.ProcessCount.Anomaly {
						log.Warnf("- Process count anomaly detected: %.1f (threshold: %.1f)", trend.ProcessCount.Mean, trend.ProcessCount.StdDev*(*anomalyThreshold))
					}

					// Log filesystem issues
					if trend.Filesystem.Critical {
						log.Warnf("- CRITICAL: Low disk space detected on one or more partitions!")
						for mount, fs := range trend.Filesystem.Partitions {
							if fs.Critical {
								log.Warnf("  * %s: Only %.1f%% free space remaining (%.2f GB)",
									mount, fs.Current, fs.Current*float64(stats.Filesystem[mount].Size)/100.0/1024.0/1024.0/1024.0)
							}
						}
					} else if trend.Filesystem.Anomaly {
						log.Warnf("- Filesystem anomaly detected:")
						for mount, fs := range trend.Filesystem.Partitions {
							if fs.Anomaly {
								if fs.Trend < 0 {
									log.Warnf("  * %s: Abnormal decrease in free space (trend: %.2f%%/sample)",
										mount, fs.Trend)
								} else {
									log.Warnf("  * %s: Abnormal change in free space (current: %.1f%%, mean: %.1f%%)",
										mount, fs.Current, fs.Mean)
								}
							}
						}
					}

					// Force crash dump creation
					crashFile := saveCrashDump(analyzer, log)
					if crashFile != "" {
						log.Warnf("Successfully created crash dump: %s", crashFile)
						s.Update(stats, nil, tempStats, crashFile)
					} else {
						log.Errorf("Failed to create crash dump!")
					}
				}
			}

			// Log current stats
			statsStr := fmt.Sprintf("=== System Stats at %s ===\n"+
				"CPU: %.1f%% user, %.1f%% system, %.1f%% idle\n"+
				"Memory: %.1f%% used (Total: %d MB, Used: %d MB, Free: %d MB)\n"+
				"Load: %.2f (1min), %.2f (5min), %.2f (15min)\n"+
				"System Stress: %.1f%%\n"+
				"Process States:\n"+
				"S: %d\n"+
				"R: %d\n"+
				"D: %d\n"+
				"Z: %d\n"+
				"Temperature:\n%s"+
				"Filesystem:\n%s"+
				"High Memory Usage Processes (>5%%):\n%s"+
				"Total CPU Usage: %.1f%%\n"+
				"Total Memory Usage: %.1f%%\n"+
				"=============================\n",
				s.Timestamp.Format(time.RFC3339),
				s.CPU.User, s.CPU.System, s.CPU.Idle,
				memUsedPct, s.Memory.Total/1024/1024, s.Memory.Used/1024/1024, s.Memory.Free/1024/1024,
				stats.LoadAverage.One, stats.LoadAverage.Five, stats.LoadAverage.Fifteen,
				s.SystemStress,
				s.Processes.Sleeping, s.Processes.Running, s.Processes.Uninterr, s.Processes.Zombie,
				getTemperatureInfo(s),
				getFilesystemInfo(stats),
				getHighMemoryProcesses(stats),
				s.CPU.User+s.CPU.System,
				memUsedPct)

			// Log to both console and file
			fmt.Print(statsStr)
			log.Print(statsStr)

			// Save summary every minute
			if time.Since(lastSummarySave) >= time.Minute {
				if err := s.Save(filepath.Join(*summaryDir, "latest.json")); err != nil {
					log.Printf("Failed to save summary: %v", err)
				}
				lastSummarySave = time.Now()
			}

		case <-snapshotTicker.C:
			// Save periodic snapshot
			filename := filepath.Join(*snapshotDir, fmt.Sprintf("snapshot-%s.json", time.Now().Format("2006-01-02-15-04-05")))
			if err := analyzer.SaveSnapshot(filename); err != nil {
				log.Errorf("Failed to save snapshot: %v", err)
			} else {
				log.Infof("Saved snapshot to %s", filename)
			}

		case sig := <-sigChan:
			log.Infof("Received signal %v, shutting down...", sig)
			return
		}
	}
}

func saveCrashDump(t *trend.TrendAnalyzer, log *logrus.Logger) string {
	// Create crash directory if it doesn't exist
	if err := os.MkdirAll(*crashDir, 0755); err != nil {
		log.Errorf("Failed to create crash directory: %v", err)
		return ""
	}

	timestamp := time.Now().Format("2006-01-02-15-04-05")
	filename := filepath.Join(*crashDir, fmt.Sprintf("crash-%s.json", timestamp))

	log.Infof("Attempting to save crash dump to %s", filename)

	if err := t.SaveSnapshot(filename); err != nil {
		log.Errorf("Failed to save crash dump: %v", err)
		return ""
	}

	// Verify file was actually created
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		log.Errorf("Crash dump file was not created at %s", filename)
		return ""
	}

	log.Infof("Successfully saved crash dump to %s", filename)
	return filename
}

func getTemperatureInfo(s *summary.SystemSummary) string {
	var result string

	if len(s.Temperature.Sensors) == 0 {
		result = "  No temperature sensors detected\n"
		return result
	}

	// Display each sensor with its max and avg
	for name, sensor := range s.Temperature.Sensors {
		result += fmt.Sprintf("  %s (%s): Current: %.1f°C, Max: %.1f°C, Avg: %.1f°C\n",
			name, sensor.Location, sensor.Value, sensor.MaxTemp, sensor.AvgTemp)
	}

	// Add overall temperature stats
	result += fmt.Sprintf("  Overall: Max: %.1f°C, Avg: %.1f°C\n", s.Temperature.MaxTemp, s.Temperature.AvgTemp)

	return result
}

func getHighMemoryProcesses(stats *parser.SystemStats) string {
	var result string
	// Create a map to deduplicate processes with the same command
	processMap := make(map[string]float64)

	for _, proc := range stats.Processes {
		if proc.VSZPercent > 5 {
			// Check if this command is already in the map
			if existingValue, ok := processMap[proc.Command]; ok {
				// Keep the higher memory usage value
				if proc.VSZPercent > existingValue {
					processMap[proc.Command] = proc.VSZPercent
				}
			} else {
				processMap[proc.Command] = proc.VSZPercent
			}
		}
	}

	// Generate output from the deduplicated map
	for cmd, memPercent := range processMap {
		result += fmt.Sprintf("%s: %.1f%%\n", cmd, memPercent)
	}

	return result
}

// Add a new function to format filesystem information
func getFilesystemInfo(stats *parser.SystemStats) string {
	var result string

	if stats.Filesystem == nil || len(stats.Filesystem) == 0 {
		result = "  No filesystem information available\n"
		return result
	}

	// Display each filesystem with its usage stats
	for mount, fs := range stats.Filesystem {
		free := 100.0 - fs.UsedPct
		status := "OK"
		if fs.Critical {
			status = "CRITICAL"
		} else if free < 20 {
			status = "WARNING"
		}

		result += fmt.Sprintf("  %s (%s): %.1f%% used, %.2f GB free [%s]\n",
			mount,
			fs.Device,
			fs.UsedPct,
			float64(fs.Available)/1024.0/1024.0/1024.0,
			status)
	}

	return result
}
