# MonChecker

A comprehensive system monitoring solution for Linux systems with LXC container support, featuring temperature monitoring, anomaly detection, and reinforcement learning. Now with enhanced ARM device support.

## Features

- Real-time system metrics collection (CPU, Memory, Network, IO)
- LXC container monitoring
- Multi-sensor temperature monitoring
  - Support for various Linux temperature sources
  - ARM device temperature support (Raspberry Pi, BeagleBone, etc.)
  - Operating temperature range tracking (-25°C to +75°C)
  - Per-sensor maximum and average temperature tracking
- Process monitoring with intelligent deduplication
- Anomaly detection with configurable thresholds
- Reinforcement learning for adaptive threshold adjustment
- Crash dump generation on anomalies
- Persistent storage of metrics and anomalies
- Cross-platform support (x86, ARM, ARM64)
- Automatic cleanup of old data

## Requirements

- Go 1.21 or later
- Linux system with LXC support
- Root privileges for system metrics collection

## Installation

1. Clone the repository:
```bash
git clone https://github.com/yourusername/monchecker.git
cd monchecker
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application for your platform:
```bash
# For x86/x64 systems
go build -o monchecker cmd/monitor/main.go

# For ARMv7 (32-bit)
GOOS=linux GOARCH=arm GOARM=7 go build -o micaCheck cmd/monitor/main.go

# For ARM64 (64-bit)
GOOS=linux GOARCH=arm64 go build -o micaCheck64 cmd/monitor/main.go
```

## Usage

### Basic Monitoring

For x86/x64 systems:
```bash
sudo ./monchecker
```

For ARM systems:
```bash
sudo ./micaCheck
```

### Using Mock Temperature Data (for testing)

For x86/x64 systems:
```bash
MOCK_TEMP=1 sudo ./monchecker
```

For ARM systems:
```bash
MOCK_TEMP=1 sudo ./micaCheck
```

### Command Line Options

```bash
# For x86/x64
sudo ./monchecker --config config.yaml --log-level debug --interval 10s --snapshot-dir /var/snapshots --crash-dir /var/crashes

# For ARM
sudo ./micaCheck --config config.yaml --log-level debug --interval 10s --snapshot-dir /var/snapshots --crash-dir /var/crashes
```

Available options:
- `--config`: Path to configuration file (default: config.yaml)
- `--log-level`: Logging level (default: info)
- `--interval`: Monitoring interval (default: 5s)
- `--snapshot-dir`: Directory for periodic snapshots (default: snapshots)
- `--crash-dir`: Directory for crash dumps (default: crashes)

### Running as a System Service

To run MonChecker as a system service (recommended for production):

```bash
# Create systemd service file
cat > /etc/systemd/system/monchecker.service << EOF
[Unit]
Description=MonChecker System Monitoring Service
After=network.target

[Service]
Type=simple
User=root
ExecStart=/path/to/monchecker --config /etc/monchecker/config.yaml
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

# Enable and start the service
systemctl enable monchecker
systemctl start monchecker
```

For ARM systems, replace the ExecStart line with:
```
ExecStart=/path/to/micaCheck --config /etc/monchecker/config.yaml
```

## Configuration

Create a `config.yaml` file with the following structure:

```yaml
collector:
  interval: 5s
  container_path: /var/lib/lxc

analyzer:
  thresholds:
    cpu_high: 80.0
    memory_high: 90.0
    io_threshold: 104857600  # 100MB
    temp_high: 70.0  # High temperature threshold (°C)
    temp_low: -20.0  # Low temperature threshold (°C)

temperature:
  enabled: true
  mock: false  # Set to true to use mock values when sensors not available

learner:
  learning_rate: 0.1
  discount: 0.95
  model_path: model.json

storage:
  metrics_dir: data/metrics
  anomalies_dir: data/anomalies
  max_file_size: 104857600  # 100MB
  max_file_age: 24h
```

## Temperature Monitoring

The system uses a multi-layered approach to detect and monitor temperature sensors:

- **Multiple Temperature Sources**
  - Standard hwmon devices
  - Thermal zones
  - ACPI thermal information
  - Device-specific paths for ARM boards

- **Per-Sensor Statistics**
  - Current temperature
  - Maximum recorded temperature
  - Average temperature over time
  - Sensor location information

- **Stress Calculation**
  - Based on operating range (-25°C to +75°C)
  - Progressive stress increases as approaching limits
  - Customized for embedded/industrial environments

## Data Storage

Metrics and anomalies are stored in JSON format in the following directories:
- `data/metrics/`: System and container metrics
- `data/anomalies/`: Detected anomalies
- `snapshots/`: Periodic system snapshots
- `crashes/`: Crash dumps generated on anomalies

Files are automatically cleaned up based on age and size limits.

## License

MIT License 