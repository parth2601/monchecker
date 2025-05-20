package trend

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/parth2601/monchecker/MonChecker/top-analyzer/pkg/parser"
)

type Trend struct {
	CPUUsage struct {
		Mean    float64
		StdDev  float64
		Trend   float64
		Anomaly bool
	}
	MemoryUsage struct {
		Mean    float64
		StdDev  float64
		Trend   float64
		Anomaly bool
	}
	ProcessCount struct {
		Mean    float64
		StdDev  float64
		Trend   float64
		Anomaly bool
	}
	Temperature struct {
		Mean              float64
		StdDev            float64
		Trend             float64
		Anomaly           bool
		Max               float64
		Min               float64
		AbsoluteThreshold float64
		ThresholdExceeded bool
		Sensors           map[string]struct {
			Mean              float64
			StdDev            float64
			Trend             float64
			Anomaly           bool
			Max               float64
			Min               float64
			AbsoluteThreshold float64
			ThresholdExceeded bool
		}
	}
	Filesystem struct {
		Partitions map[string]struct {
			Mean       float64 // Mean free space percentage
			StdDev     float64
			Trend      float64 // Trend of free space (negative means decreasing)
			Anomaly    bool
			Min        float64 // Minimum free space percentage observed
			Max        float64 // Maximum free space percentage observed
			Current    float64 // Current free space percentage
			Critical   bool    // Less than 10% free space
			Device     string
			MountPoint string
		}
		Anomaly  bool // Any partition has anomaly
		Critical bool // Any partition is critical
	}
	SystemStress float64
}

type TrendAnalyzer struct {
	history             []*parser.SystemStats
	window              int
	tempHistory         map[string][]float64
	longTermTempHistory map[string][]float64
	anomalyThreshold    float64
	trendThreshold      float64
	tempThreshold       float64
	longTermWindow      int
}

func New(window int) *TrendAnalyzer {
	return &TrendAnalyzer{
		history:             make([]*parser.SystemStats, 0, window),
		window:              window,
		tempHistory:         make(map[string][]float64),
		longTermTempHistory: make(map[string][]float64),
		anomalyThreshold:    2.0,
		trendThreshold:      0.1,
		tempThreshold:       70.0,
		longTermWindow:      100,
	}
}

// NewWithOptions creates a new TrendAnalyzer with configurable options
func NewWithOptions(window int, anomalyThreshold float64, trendThreshold float64) *TrendAnalyzer {
	return &TrendAnalyzer{
		history:             make([]*parser.SystemStats, 0, window),
		window:              window,
		tempHistory:         make(map[string][]float64),
		longTermTempHistory: make(map[string][]float64),
		anomalyThreshold:    anomalyThreshold,
		trendThreshold:      trendThreshold,
		tempThreshold:       70.0,
		longTermWindow:      window * 10,
	}
}

// Enhanced version with temperature threshold configuration
func NewWithFullOptions(window int, anomalyThreshold float64, trendThreshold float64, tempThreshold float64, longTermWindow int) *TrendAnalyzer {
	return &TrendAnalyzer{
		history:             make([]*parser.SystemStats, 0, window),
		window:              window,
		tempHistory:         make(map[string][]float64),
		longTermTempHistory: make(map[string][]float64),
		anomalyThreshold:    anomalyThreshold,
		trendThreshold:      trendThreshold,
		tempThreshold:       tempThreshold,
		longTermWindow:      longTermWindow,
	}
}

func (t *TrendAnalyzer) AddStats(stats *parser.SystemStats) {
	t.history = append(t.history, stats)
	if len(t.history) > t.window {
		t.history = t.history[1:]
	}

	// Update temperature history for each sensor
	for name, temp := range stats.Temperature.Sensors {
		// Update regular temperature history
		if _, exists := t.tempHistory[name]; !exists {
			t.tempHistory[name] = make([]float64, 0, t.window)
		}
		t.tempHistory[name] = append(t.tempHistory[name], temp)
		if len(t.tempHistory[name]) > t.window {
			t.tempHistory[name] = t.tempHistory[name][1:]
		}

		// Update long-term temperature history
		if _, exists := t.longTermTempHistory[name]; !exists {
			t.longTermTempHistory[name] = make([]float64, 0, t.longTermWindow)
		}
		t.longTermTempHistory[name] = append(t.longTermTempHistory[name], temp)
		if len(t.longTermTempHistory[name]) > t.longTermWindow {
			t.longTermTempHistory[name] = t.longTermTempHistory[name][1:]
		}
	}
}

func (t *TrendAnalyzer) Analyze() *Trend {
	if len(t.history) < 2 {
		return nil
	}

	trend := &Trend{
		Temperature: struct {
			Mean              float64
			StdDev            float64
			Trend             float64
			Anomaly           bool
			Max               float64
			Min               float64
			AbsoluteThreshold float64
			ThresholdExceeded bool
			Sensors           map[string]struct {
				Mean              float64
				StdDev            float64
				Trend             float64
				Anomaly           bool
				Max               float64
				Min               float64
				AbsoluteThreshold float64
				ThresholdExceeded bool
			}
		}{
			AbsoluteThreshold: t.tempThreshold,
			Sensors: make(map[string]struct {
				Mean              float64
				StdDev            float64
				Trend             float64
				Anomaly           bool
				Max               float64
				Min               float64
				AbsoluteThreshold float64
				ThresholdExceeded bool
			}),
		},
		Filesystem: struct {
			Partitions map[string]struct {
				Mean       float64
				StdDev     float64
				Trend      float64
				Anomaly    bool
				Min        float64
				Max        float64
				Current    float64
				Critical   bool
				Device     string
				MountPoint string
			}
			Anomaly  bool
			Critical bool
		}{
			Partitions: make(map[string]struct {
				Mean       float64
				StdDev     float64
				Trend      float64
				Anomaly    bool
				Min        float64
				Max        float64
				Current    float64
				Critical   bool
				Device     string
				MountPoint string
			}),
			Anomaly:  false,
			Critical: false,
		},
	}

	// Calculate CPU usage trend
	cpuUsages := make([]float64, len(t.history))
	for i, stats := range t.history {
		cpuUsages[i] = stats.CPU.User + stats.CPU.Sys
	}
	trend.CPUUsage.Mean, trend.CPUUsage.StdDev = calculateStats(cpuUsages)
	trend.CPUUsage.Trend = calculateTrend(cpuUsages)
	trend.CPUUsage.Anomaly = detectAnomalyWithThreshold(cpuUsages, trend.CPUUsage.Mean, trend.CPUUsage.StdDev, t.anomalyThreshold) ||
		detectTrendAnomaly(trend.CPUUsage.Trend, t.trendThreshold)

	// Calculate memory usage trend
	memUsages := make([]float64, len(t.history))
	for i, stats := range t.history {
		if stats.Memory.Total > 0 {
			memUsages[i] = float64(stats.Memory.Used) / float64(stats.Memory.Total) * 100
		} else {
			memUsages[i] = 0
		}
	}
	trend.MemoryUsage.Mean, trend.MemoryUsage.StdDev = calculateStats(memUsages)
	trend.MemoryUsage.Trend = calculateTrend(memUsages)
	trend.MemoryUsage.Anomaly = detectAnomalyWithThreshold(memUsages, trend.MemoryUsage.Mean, trend.MemoryUsage.StdDev, t.anomalyThreshold) ||
		detectTrendAnomaly(trend.MemoryUsage.Trend, t.trendThreshold)

	// Calculate process count trend
	procCounts := make([]float64, len(t.history))
	for i, stats := range t.history {
		procCounts[i] = float64(len(stats.Processes))
	}
	trend.ProcessCount.Mean, trend.ProcessCount.StdDev = calculateStats(procCounts)
	trend.ProcessCount.Trend = calculateTrend(procCounts)
	trend.ProcessCount.Anomaly = detectAnomalyWithThreshold(procCounts, trend.ProcessCount.Mean, trend.ProcessCount.StdDev, t.anomalyThreshold) ||
		detectTrendAnomaly(trend.ProcessCount.Trend, t.trendThreshold)

	// Calculate temperature trends for each sensor
	allTemps := make([]float64, 0)
	maxTemps := make([]float64, 0)
	avgTemps := make([]float64, 0)

	for name, temps := range t.tempHistory {
		if len(temps) > 0 {
			mean, stddev := calculateStats(temps)
			trendValue := calculateTrend(temps)

			// Check long-term trend if available
			longTermTrend := 0.0
			if longTermTemps, exists := t.longTermTempHistory[name]; exists && len(longTermTemps) > 10 {
				longTermTrend = calculateTrend(longTermTemps)
			}

			sensorStats := struct {
				Mean              float64
				StdDev            float64
				Trend             float64
				Anomaly           bool
				Max               float64
				Min               float64
				AbsoluteThreshold float64
				ThresholdExceeded bool
			}{
				Mean:              mean,
				StdDev:            stddev,
				Trend:             trendValue,
				AbsoluteThreshold: t.tempThreshold,
				Max:               temps[0],
				Min:               temps[0],
			}

			// Calculate min/max from history
			for _, temp := range temps {
				if temp < sensorStats.Min {
					sensorStats.Min = temp
				}
				if temp > sensorStats.Max {
					sensorStats.Max = temp
				}
			}

			// Check if max temperature exceeds absolute threshold
			sensorStats.ThresholdExceeded = sensorStats.Max > t.tempThreshold

			// Detect anomalies using both Z-score and trend
			sensorStats.Anomaly = detectAnomalyWithThreshold(temps, mean, stddev, t.anomalyThreshold) ||
				detectTrendAnomaly(trendValue, t.trendThreshold) ||
				detectTrendAnomaly(longTermTrend, t.trendThreshold*0.5) ||
				sensorStats.ThresholdExceeded

			trend.Temperature.Sensors[name] = sensorStats
			allTemps = append(allTemps, temps...)
			maxTemps = append(maxTemps, sensorStats.Max)
			avgTemps = append(avgTemps, sensorStats.Mean)

			// Update overall temperature stats
			if sensorStats.Max > trend.Temperature.Max {
				trend.Temperature.Max = sensorStats.Max
			}
			if trend.Temperature.Min == 0 || sensorStats.Min < trend.Temperature.Min {
				trend.Temperature.Min = sensorStats.Min
			}

			// Update overall threshold exceeded flag
			if sensorStats.ThresholdExceeded {
				trend.Temperature.ThresholdExceeded = true
			}
		}
	}

	// Calculate overall temperature stats from all sensor history
	if len(allTemps) > 0 {
		trend.Temperature.Mean, trend.Temperature.StdDev = calculateStats(allTemps)
		tempTrendValue := calculateTrend(allTemps)
		trend.Temperature.Trend = tempTrendValue

		// Calculate long-term trend for all temperatures combined
		allLongTermTemps := make([]float64, 0)
		for _, longTermTemps := range t.longTermTempHistory {
			allLongTermTemps = append(allLongTermTemps, longTermTemps...)
		}

		longTermTrend := 0.0
		if len(allLongTermTemps) > 10 {
			longTermTrend = calculateTrend(allLongTermTemps)
		}

		// Detect temperature anomalies using both methods and threshold check
		trend.Temperature.Anomaly = detectAnomalyWithThreshold(allTemps, trend.Temperature.Mean, trend.Temperature.StdDev, t.anomalyThreshold) ||
			detectTrendAnomaly(tempTrendValue, t.trendThreshold) ||
			detectTrendAnomaly(longTermTrend, t.trendThreshold*0.5) || // More sensitive for long-term
			trend.Temperature.ThresholdExceeded
	}

	// Calculate max and average from all sensors
	if len(maxTemps) > 0 {
		trend.Temperature.Max = maxTemps[0]
		for _, temp := range maxTemps {
			if temp > trend.Temperature.Max {
				trend.Temperature.Max = temp
			}
		}
		// Check if max temperature exceeds threshold
		trend.Temperature.ThresholdExceeded = trend.Temperature.Max > t.tempThreshold
	}

	if len(avgTemps) > 0 {
		sum := 0.0
		for _, temp := range avgTemps {
			sum += temp
		}
		trend.Temperature.Mean = sum / float64(len(avgTemps))
	}

	// Calculate filesystem space trends
	if len(t.history) > 0 && t.history[len(t.history)-1].Filesystem != nil {
		// Map to track partition history across time
		fsHistory := make(map[string][]float64)

		// First collect historical data for each partition
		for _, stats := range t.history {
			if stats.Filesystem == nil {
				continue
			}

			for mountPoint, fs := range stats.Filesystem {
				if _, exists := fsHistory[mountPoint]; !exists {
					fsHistory[mountPoint] = make([]float64, 0, len(t.history))
				}

				// Store free space percentage
				freePercent := 100.0 - fs.UsedPct
				fsHistory[mountPoint] = append(fsHistory[mountPoint], freePercent)
			}
		}

		// Now analyze each partition
		for mountPoint, freeSpaceHistory := range fsHistory {
			if len(freeSpaceHistory) < 2 {
				continue
			}

			// Get current filesystem stats
			currentFs := t.history[len(t.history)-1].Filesystem[mountPoint]

			mean, stddev := calculateStats(freeSpaceHistory)
			trendValue := calculateTrend(freeSpaceHistory)
			current := 100.0 - currentFs.UsedPct

			// Find min/max free space
			min, max := freeSpaceHistory[0], freeSpaceHistory[0]
			for _, free := range freeSpaceHistory {
				if free < min {
					min = free
				}
				if free > max {
					max = free
				}
			}

			// Detect anomalies
			anomaly := detectAnomalyWithThreshold(freeSpaceHistory, mean, stddev, t.anomalyThreshold) ||
				detectTrendAnomaly(trendValue, t.trendThreshold*2) // More sensitive for filesystem trends

			// Detect critical state (less than 10% free)
			critical := current < 10.0

			// Store partition stats
			partitionStats := struct {
				Mean       float64
				StdDev     float64
				Trend      float64
				Anomaly    bool
				Min        float64
				Max        float64
				Current    float64
				Critical   bool
				Device     string
				MountPoint string
			}{
				Mean:       mean,
				StdDev:     stddev,
				Trend:      trendValue,
				Anomaly:    anomaly,
				Min:        min,
				Max:        max,
				Current:    current,
				Critical:   critical,
				Device:     currentFs.Device,
				MountPoint: mountPoint,
			}

			trend.Filesystem.Partitions[mountPoint] = partitionStats

			// Update overall filesystem status
			if anomaly {
				trend.Filesystem.Anomaly = true
			}
			if critical {
				trend.Filesystem.Critical = true
			}
		}
	}

	// Calculate system stress
	trend.SystemStress = calculateSystemStress(trend)

	return trend
}

func calculateSystemStress(trend *Trend) float64 {
	risk := 0.0

	// CPU stress factors
	if trend.CPUUsage.Mean > 20 {
		risk += 20
	} else if trend.CPUUsage.Mean > 10 {
		risk += 10
	}

	// Memory stress factors
	if trend.MemoryUsage.Mean > 90 {
		risk += 30
	} else if trend.MemoryUsage.Mean > 80 {
		risk += 20
	} else if trend.MemoryUsage.Mean > 70 {
		risk += 10
	}

	// Process count stress factors
	if trend.ProcessCount.Mean > 100 {
		risk += 20
	} else if trend.ProcessCount.Mean > 50 {
		risk += 10
	}

	// Uninterruptible processes stress
	if trend.ProcessCount.Anomaly {
		risk += 20
	}

	// High CPU processes stress
	if trend.CPUUsage.Anomaly {
		risk += 20
	}

	// Temperature stress - Operating range: -25°C to 75°C
	// Use the ThresholdExceeded flag instead of hardcoded temperature limits
	if trend.Temperature.ThresholdExceeded {
		// Approaching the upper limit of operating range
		risk += 50
	} else if trend.Temperature.Max > 60 {
		risk += 20
	} else if trend.Temperature.Max > 50 {
		risk += 10
	} else if trend.Temperature.Max < -20 {
		// Approaching the lower limit of operating range
		risk += 20
	} else if trend.Temperature.Max < -10 {
		risk += 10
	}

	// Add stress for temperature anomalies detected by trend analysis
	if trend.Temperature.Anomaly && !trend.Temperature.ThresholdExceeded {
		risk += 15 // Add some risk, but less than threshold violation
	}

	// Filesystem stress factors
	if trend.Filesystem.Critical {
		// Critical disk space situation (less than 10% free on any partition)
		risk += 40
	} else if trend.Filesystem.Anomaly {
		// Anomalous disk space trends detected
		risk += 20
	}

	// Add stress for individual critical partitions, especially root and boot
	for mountPoint, fs := range trend.Filesystem.Partitions {
		if fs.Critical {
			// Higher stress for critical system partitions
			if mountPoint == "/" {
				risk += 30 // Root partition critical
			} else if mountPoint == "/boot" {
				risk += 25 // Boot partition critical
			} else {
				risk += 15 // Other partition critical
			}
		} else if fs.Current < 20 {
			// Warning level (less than 20% free)
			if mountPoint == "/" {
				risk += 15 // Root partition low
			} else if mountPoint == "/boot" {
				risk += 10 // Boot partition low
			} else {
				risk += 5 // Other partition low
			}
		}

		// Negative trend in free space is also a concern
		if fs.Trend < -1.0 {
			// Rapidly decreasing free space
			risk += 15
		} else if fs.Trend < -0.5 {
			// Moderately decreasing free space
			risk += 5
		}
	}

	return min(risk, 100.0)
}

func (t *TrendAnalyzer) SaveSnapshot(filename string) error {
	// Create a copy of history with deduplicated processes to avoid redundancy in crash dumps
	deduplicatedHistory := make([]*parser.SystemStats, len(t.history))

	// Deep copy with process deduplication
	for i, stats := range t.history {
		// Copy the stats
		newStats := &parser.SystemStats{
			Memory:      stats.Memory,
			CPU:         stats.CPU,
			LoadAverage: stats.LoadAverage,
			Temperature: stats.Temperature,
			Filesystem:  make(map[string]parser.FilesystemStats),
			// We'll deduplicate processes
			Processes: make([]parser.Process, 0),
		}

		// Copy filesystem stats
		for mountPoint, fs := range stats.Filesystem {
			newStats.Filesystem[mountPoint] = fs
		}

		// Deduplicate processes by command
		processMap := make(map[string]parser.Process)
		for _, proc := range stats.Processes {
			if existingProc, ok := processMap[proc.Command]; ok {
				// Keep the process with higher VSZPercent or CPU
				if proc.VSZPercent > existingProc.VSZPercent || proc.CPUPercent > existingProc.CPUPercent {
					processMap[proc.Command] = proc
				}
			} else {
				processMap[proc.Command] = proc
			}
		}

		// Add deduplicated processes back to the stats
		for _, proc := range processMap {
			newStats.Processes = append(newStats.Processes, proc)
		}

		deduplicatedHistory[i] = newStats
	}

	data := struct {
		Timestamp time.Time
		Stats     []*parser.SystemStats
		Trend     *Trend
		Summary   struct {
			TotalStorage       int64
			UsedStorage        int64
			FreeStorage        int64
			StorageUsagePct    float64
			CriticalPartitions []string
			LowSpacePartitions []string
		}
	}{
		Timestamp: time.Now(),
		Stats:     deduplicatedHistory,
		Trend:     t.Analyze(),
	}

	// Calculate storage summary from latest stats
	if len(deduplicatedHistory) > 0 {
		latest := deduplicatedHistory[len(deduplicatedHistory)-1]
		data.Summary.TotalStorage = 0
		data.Summary.UsedStorage = 0
		data.Summary.FreeStorage = 0

		for mountPoint, fs := range latest.Filesystem {
			data.Summary.TotalStorage += fs.Size
			data.Summary.UsedStorage += fs.Used
			data.Summary.FreeStorage += fs.Available

			// Track critical and low space partitions
			if fs.Critical {
				data.Summary.CriticalPartitions = append(data.Summary.CriticalPartitions, mountPoint)
			} else if fs.UsedPct > 80 {
				data.Summary.LowSpacePartitions = append(data.Summary.LowSpacePartitions, mountPoint)
			}
		}

		// Calculate overall storage usage percentage
		if data.Summary.TotalStorage > 0 {
			data.Summary.StorageUsagePct = float64(data.Summary.UsedStorage) / float64(data.Summary.TotalStorage) * 100
		}
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	return nil
}

func calculateStats(values []float64) (mean, stdDev float64) {
	if len(values) == 0 {
		return 0, 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean = sum / float64(len(values))

	// Calculate standard deviation
	sumSq := 0.0
	for _, v := range values {
		diff := v - mean
		sumSq += diff * diff
	}
	stdDev = sqrt(sumSq / float64(len(values)))

	return mean, stdDev
}

func calculateTrend(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}

	// Simple linear regression
	n := float64(len(values))
	sumX, sumY, sumXY, sumX2 := 0.0, 0.0, 0.0, 0.0

	for i, y := range values {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}

	slope := (n*sumXY - sumX*sumY) / (n*sumX2 - sumX*sumX)
	return slope
}

// detectAnomalyWithThreshold checks if the latest value is an anomaly based on the given threshold
func detectAnomalyWithThreshold(values []float64, mean, stdDev, threshold float64) bool {
	if len(values) == 0 || stdDev == 0 {
		return false
	}

	lastValue := values[len(values)-1]
	zScore := (lastValue - mean) / stdDev

	// Consider it an anomaly if it's more than the threshold standard deviations from the mean
	return zScore > threshold || zScore < -threshold
}

// detectTrendAnomaly checks if the trend (slope) exceeds the given threshold
func detectTrendAnomaly(trend float64, trendThreshold float64) bool {
	return math.Abs(trend) > trendThreshold
}

func sqrt(x float64) float64 {
	// Simple square root implementation
	z := 1.0
	for i := 0; i < 10; i++ {
		z -= (z*z - x) / (2 * z)
	}
	return z
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (t *TrendAnalyzer) GetCurrentStats() *parser.SystemStats {
	if len(t.history) == 0 {
		return nil
	}
	return t.history[len(t.history)-1]
}
