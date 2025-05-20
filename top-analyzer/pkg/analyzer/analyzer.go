package analyzer

import (
	"fmt"
	"runtime"
	"sort"
	"time"

	"github.com/parth2601/monchecker/MonChecker/top-analyzer/pkg/parser"
)

type Insight struct {
	Type        string
	Description string
	Severity    string
	Timestamp   time.Time
}

type Analyzer struct {
	history []*parser.SystemStats
	maxHistory int
}

func New(maxHistory int) *Analyzer {
	return &Analyzer{
		history: make([]*parser.SystemStats, 0),
		maxHistory: maxHistory,
	}
}

func (a *Analyzer) AddStats(stats *parser.SystemStats) {
	a.history = append(a.history, stats)
	if len(a.history) > a.maxHistory {
		a.history = a.history[1:]
	}
}

func (a *Analyzer) GetInsights() []Insight {
	if len(a.history) == 0 {
		return nil
	}

	current := a.history[len(a.history)-1]
	insights := make([]Insight, 0)

	// CPU Usage Insights
	if current.CPU.User + current.CPU.Sys > 80 {
		insights = append(insights, Insight{
			Type:        "High CPU Usage",
			Description: fmt.Sprintf("CPU usage is high: %.1f%% user, %.1f%% system", current.CPU.User, current.CPU.Sys),
			Severity:    "Warning",
			Timestamp:   time.Now(),
		})
	}

	// Memory Usage Insights
	totalMem := current.Memory.Used + current.Memory.Free + current.Memory.Shared + current.Memory.Buffers + current.Memory.Cached
	memUsagePercent := float64(current.Memory.Used) / float64(totalMem) * 100
	if memUsagePercent > 90 {
		insights = append(insights, Insight{
			Type:        "High Memory Usage",
			Description: fmt.Sprintf("Memory usage is high: %.1f%%", memUsagePercent),
			Severity:    "Warning",
			Timestamp:   time.Now(),
		})
	}

	// Load Average Insights
	if current.LoadAverage.One > float64(runtime.NumCPU())*2 {
		insights = append(insights, Insight{
			Type:        "High System Load",
			Description: fmt.Sprintf("System load is high: %.2f (1min)", current.LoadAverage.One),
			Severity:    "Warning",
			Timestamp:   time.Now(),
		})
	}

	// Process Insights
	topProcesses := a.getTopProcesses(current, 5)
	for _, proc := range topProcesses {
		if proc.CPUPercent > 50 {
			insights = append(insights, Insight{
				Type:        "High CPU Process",
				Description: fmt.Sprintf("Process %s (PID: %d) using %.1f%% CPU", proc.Command, proc.PID, proc.CPUPercent),
				Severity:    "Info",
				Timestamp:   time.Now(),
			})
		}
		if proc.VSZPercent > 10 {
			insights = append(insights, Insight{
				Type:        "High Memory Process",
				Description: fmt.Sprintf("Process %s (PID: %d) using %.1f%% memory", proc.Command, proc.PID, proc.VSZPercent),
				Severity:    "Info",
				Timestamp:   time.Now(),
			})
		}
	}

	// Trend Analysis
	if len(a.history) > 1 {
		prev := a.history[len(a.history)-2]
		cpuTrend := (current.CPU.User + current.CPU.Sys) - (prev.CPU.User + prev.CPU.Sys)
		if cpuTrend > 20 {
			insights = append(insights, Insight{
				Type:        "CPU Usage Spike",
				Description: fmt.Sprintf("CPU usage increased by %.1f%%", cpuTrend),
				Severity:    "Warning",
				Timestamp:   time.Now(),
			})
		}
	}

	return insights
}

func (a *Analyzer) getTopProcesses(stats *parser.SystemStats, count int) []parser.Process {
	processes := make([]parser.Process, len(stats.Processes))
	copy(processes, stats.Processes)

	// Sort by CPU usage
	sort.Slice(processes, func(i, j int) bool {
		return processes[i].CPUPercent > processes[j].CPUPercent
	})

	if len(processes) > count {
		return processes[:count]
	}
	return processes
} 
