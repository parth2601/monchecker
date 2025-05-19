# Top Analyzer

A comprehensive system monitoring and analysis tool that provides detailed process analysis, deadlock detection, anomaly monitoring, temperature tracking, and power utilization monitoring. Optimized for various systems including ARM-based devices.

## Features

- Real-time system monitoring
- Detailed process analysis with intelligent deduplication
- Deadlock risk detection
- Anomaly detection and trend analysis
- Comprehensive temperature monitoring
  - Multi-sensor support with per-sensor max/avg tracking
  - Support for various Linux temperature sources
  - ARM device temperature support (Raspberry Pi, BeagleBone, etc.)
  - Operating temperature range (-25°C to +75°C) with appropriate stress calculation
- Power utilization monitoring
- Automatic crash dumps with deduplication
- Configurable monitoring periods
- Cross-platform support (x86, ARM, ARM64)

## Installation

### Building from Source (x86/x64)
```bash
git clone https://github.com/monchecker/top-analyzer.git
cd top-analyzer
go build -o top-analyzer cmd/analyzer/main.go
```

### Cross-compiling for ARM
```bash
# For ARMv7 (32-bit)
GOOS=linux GOARCH=arm GOARM=7 go build -o micaCheck cmd/analyzer/main.go

# For ARM64 (64-bit)
GOOS=linux GOARCH=arm64 go build -o micaCheck64 cmd/analyzer/main.go
```

## Usage

### Basic Monitoring (x86/x64)
```bash
./top-analyzer
```

### Basic Monitoring (ARM)
```bash
./micaCheck
```

### Custom Monitoring Intervals
```bash
# For x86/x64
./top-analyzer -interval 10s -history 20

# For ARM
./micaCheck -interval 10s -history 20
```

### Custom Snapshot Period
```bash
# For x86/x64
./top-analyzer -snapshot-period 30m

# For ARM
./micaCheck -snapshot-period 30m
```

### Custom Directories
```bash
# For x86/x64
./top-analyzer -snapshot-dir /var/snapshots -crash-dir /var/crashes -summary-dir /var/summary

# For ARM
./micaCheck -snapshot-dir /var/snapshots -crash-dir /var/crashes -summary-dir /var/summary
```

### Using Mock Temperature Data (for testing)
```bash
# For x86/x64
MOCK_TEMP=1 ./top-analyzer

# For ARM
MOCK_TEMP=1 ./micaCheck
```

### Running as a Service
To run as a system service (recommended for production):

```bash
# Create systemd service file
cat > /etc/systemd/system/top-analyzer.service << EOF
[Unit]
Description=Top Analyzer Monitoring Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/path/to/top-analyzer -interval 5s -log /var/log/top-analyzer.log
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
systemctl enable top-analyzer
systemctl start top-analyzer
```

## Configuration Options

| Flag | Default | Description |
|------|---------|-------------|
| `-interval` | 5s | Interval between top command executions |
| `-history` | 10 | Number of samples to keep in history |
| `-log` | top-analyzer.log | Path to log file |
| `-snapshot-dir` | snapshots | Directory for snapshots |
| `-crash-dir` | crashes | Directory for crash dumps |
| `-summary-dir` | summary | Directory for summary files |
| `-snapshot-period` | 1h | Period between snapshots |
| `-anomaly-threshold` | 3.5 | Z-score threshold for anomaly detection |

## Analysis Components

### 1. Process Analysis
- Tracks all processes with intelligent deduplication
- Groups processes by state (R, S, D, Z)
- Identifies high CPU (>10%) and memory (>5%) processes
- Calculates total CPU and memory usage
- Eliminates redundant process entries in logs and displays

### 2. Temperature Monitoring
The system uses a multi-layered approach to detect and monitor temperature sensors:

- **Multiple Temperature Sources**
  - Standard hwmon devices (`/sys/class/hwmon/hwmon*`)
  - Thermal zones (`/sys/class/thermal/thermal_zone*`)
  - ACPI thermal information (`/proc/acpi/thermal_zone/`)
  - Device-specific paths for ARM boards

- **Per-Sensor Statistics**
  - Current temperature
  - Maximum recorded temperature
  - Average temperature over time
  - Sensor location information

- **Overall Temperature Metrics**
  - System-wide maximum temperature
  - System-wide average temperature
  - Temperature trend analysis

- **Stress Calculation**
  - Based on operating range (-25°C to +75°C)
  - Progressive stress increases as approaching limits
  - Customized for embedded/industrial environments

### 3. Deadlock Risk Calculation
Multiple factors contribute to deadlock risk (0-100 scale):

- **High CPU with Low IO** (30 risk)
  - CPU usage > 90%
  - IO wait < 1%

- **Memory Pressure** (30 risk)
  - Memory usage > 90%

- **High Load Average** (30 risk)
  - Load > 10 for 1-minute average

- **High Number of D-State Processes** (20 risk)
  - Processes in uninterruptible sleep
  - More than 5 D-state processes

- **Multiple High-CPU Processes** (20 risk)
  - Same command using high CPU
  - Multiple instances

- **Temperature Stress** (30 risk)
  - Temperature approaching operating limits

### 4. Anomaly Detection
Uses statistical analysis to detect anomalies:
- Mean and standard deviation of historical data
- Z-score calculation for current values
- Anomaly if value is >2 standard deviations from mean
- Triggers crash dumps when anomalies are detected

## Output Interpretation

### Process States
- `R`: Running
- `S`: Sleeping/Suspended
- `D`: Uninterruptible sleep (potential deadlock indicator)
- `Z`: Zombie (potential deadlock indicator; Alive and not active for a while)

### Stress Levels
- **0-30**: Low stress
- **31-60**: Medium stress
- **61-84**: High stress
- **85-100**: Critical stress (triggers crash dump)

### Temperature Ranges
- **< -20°C**: Below recommended operating range
- **-20°C to 70°C**: Normal operating range
- **> 70°C**: Above recommended operating range

### Anomalies Detected
- CPU usage spikes
- Memory usage spikes
- Process count changes
- Temperature anomalies

## Crash Dumps
Generated when any of the following conditions are met:
- System stress ≥ 85%
- CPU usage anomaly detected
- Memory usage anomaly detected
- Temperature anomaly detected
- Program panic
- Manual trigger

Contains:
- Current system state with deduplicated processes
- Per-sensor temperature data
- Trend analysis
- Historical data

## Use Cases

1. **Resource Bottleneck Detection**
   - Identify CPU-intensive processes
   - Track memory leaks
   - Monitor system load

2. **Thermal Monitoring**
   - Track temperature across multiple sensors
   - Identify overheating components
   - Monitor thermal trends over time

3. **Process Contention Analysis**
   - Detect competing processes
   - Identify resource conflicts
   - Monitor process states

4. **System Stability Monitoring**
   - Early deadlock detection
   - Anomaly identification
   - Performance trend analysis

5. **Embedded System Monitoring**
   - ARM device monitoring
   - Temperature tracking in industrial environments
   - Resource usage in constrained environments

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details. 