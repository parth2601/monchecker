package temperature

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type TemperatureStats struct {
	Sensors map[string]float64 // sensor name -> temperature in Celsius
}

func ReadTemperatureStats() (*TemperatureStats, error) {
	stats := &TemperatureStats{
		Sensors: make(map[string]float64),
	}

	// Try multiple temperature source paths
	// First try standard hwmon
	if err := readFromHwmon(stats); err == nil && len(stats.Sensors) > 0 {
		return stats, nil
	}

	// Then try thermal_zone (common on ARM devices)
	if err := readFromThermalZone(stats); err == nil && len(stats.Sensors) > 0 {
		return stats, nil
	}

	// Fallback to procfs if available
	if err := readFromProcTemperature(stats); err == nil && len(stats.Sensors) > 0 {
		return stats, nil
	}

	// Last resort: manually check known device-specific files
	// This is very device specific but can help on certain ARM boards
	if err := readFromDeviceSpecific(stats); err == nil && len(stats.Sensors) > 0 {
		return stats, nil
	}

	// If we have at least one sensor, return success even if some methods failed
	if len(stats.Sensors) > 0 {
		return stats, nil
	}

	// No sensors found, create mock values for testing
	if os.Getenv("MOCK_TEMP") == "1" {
		stats.Sensors["cpu"] = 45.5
		stats.Sensors["board"] = 38.2
		return stats, nil
	}

	return stats, fmt.Errorf("no temperature sensors found")
}

func readFromHwmon(stats *TemperatureStats) error {
	// Read all hwmon devices
	hwmonDirs, err := filepath.Glob("/sys/class/hwmon/hwmon*")
	if err != nil {
		return fmt.Errorf("failed to find hwmon devices: %w", err)
	}

	for _, hwmonDir := range hwmonDirs {
		// Get sensor name
		nameBytes, err := ioutil.ReadFile(filepath.Join(hwmonDir, "name"))
		if err != nil {
			continue
		}
		name := strings.TrimSpace(string(nameBytes))

		// Read temperature
		tempFiles, err := filepath.Glob(filepath.Join(hwmonDir, "temp*_input"))
		if err != nil {
			continue
		}

		for _, tempFile := range tempFiles {
			tempBytes, err := ioutil.ReadFile(tempFile)
			if err != nil {
				continue
			}

			temp, err := strconv.ParseFloat(strings.TrimSpace(string(tempBytes)), 64)
			if err != nil {
				continue
			}

			// Convert millidegree Celsius to Celsius
			stats.Sensors[name] = temp / 1000.0
		}
	}

	return nil
}

func readFromThermalZone(stats *TemperatureStats) error {
	// Try thermal_zone directories
	thermalDirs, err := filepath.Glob("/sys/class/thermal/thermal_zone*")
	if err != nil {
		return fmt.Errorf("failed to find thermal zones: %w", err)
	}

	for _, dir := range thermalDirs {
		// Get zone type (name)
		typeBytes, err := ioutil.ReadFile(filepath.Join(dir, "type"))
		if err != nil {
			continue
		}
		zoneType := strings.TrimSpace(string(typeBytes))

		// Read temperature
		tempBytes, err := ioutil.ReadFile(filepath.Join(dir, "temp"))
		if err != nil {
			continue
		}

		temp, err := strconv.ParseFloat(strings.TrimSpace(string(tempBytes)), 64)
		if err != nil {
			continue
		}

		// Convert millidegree Celsius to Celsius
		stats.Sensors[zoneType] = temp / 1000.0
	}

	return nil
}

func readFromProcTemperature(stats *TemperatureStats) error {
	// Try to read from /proc/acpi/thermal_zone if it exists
	files, err := filepath.Glob("/proc/acpi/thermal_zone/*/temperature")
	if err != nil {
		return fmt.Errorf("failed to check thermal zones: %w", err)
	}

	for _, file := range files {
		// Extract zone name from path
		zoneName := filepath.Base(filepath.Dir(file))

		data, err := ioutil.ReadFile(file)
		if err != nil {
			continue
		}

		// Parse temperature value
		parts := strings.Fields(string(data))
		if len(parts) < 2 {
			continue
		}

		temp, err := strconv.ParseFloat(parts[1], 64)
		if err != nil {
			continue
		}

		stats.Sensors[zoneName] = temp
	}

	return nil
}

func readFromDeviceSpecific(stats *TemperatureStats) error {
	// Check for Raspberry Pi temperature
	piTempFile := "/sys/class/thermal/thermal_zone0/temp"
	if _, err := os.Stat(piTempFile); err == nil {
		data, err := ioutil.ReadFile(piTempFile)
		if err == nil {
			temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
			if err == nil {
				stats.Sensors["rpi_cpu"] = temp / 1000.0
			}
		}
	}

	// Check for BeagleBone temperature
	bbTempFiles := []string{
		"/sys/devices/platform/omap/44e10800.i2c/i2c-0/0-0050/temp1_input",
		"/sys/devices/platform/44e10800.i2c/i2c-0/0-0050/temp1_input",
	}

	for _, file := range bbTempFiles {
		if _, err := os.Stat(file); err == nil {
			data, err := ioutil.ReadFile(file)
			if err == nil {
				temp, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
				if err == nil {
					stats.Sensors["beaglebone"] = temp / 1000.0
				}
			}
		}
	}

	return nil
}

func (t *TemperatureStats) String() string {
	var sb strings.Builder
	sb.WriteString("Temperature Statistics:\n")
	for name, temp := range t.Sensors {
		sb.WriteString(fmt.Sprintf("%s: %.1f°C\n", name, temp))
	}
	return sb.String()
}
