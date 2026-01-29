# Printer Control Integration

`hkcam` now supports 3D printer control integration, allowing you to monitor and control your printer directly from HomeKit. This feature is based on the Python PoC implementation and provides seamless integration with printers that support G-code commands over serial connection.

## Features

The printer control integration adds the following HomeKit services to your camera accessory:

### 1. Print Control Switch
- **Pause/Resume printing** via a HomeKit switch
- ON = actively printing (not paused)
- OFF = paused or not printing

### 2. Bed Temperature Thermostat
- **Monitor** current bed temperature
- **Set target** bed temperature (0 to configurable max, default 110°C)
- **Control heating mode** (Off/Heat)
- Temperature displayed in Celsius

### 3. Nozzle Temperature Thermostat
- **Monitor** current nozzle temperature
- **Set target** nozzle temperature (0 to configurable max, default 260°C)
- **Control heating mode** (Off/Heat)
- Temperature displayed in Celsius

### 4. Part Cooling Fan
- **Control fan** on/off state
- Automatically turns off when print is cancelled (temps go to 0)

## Requirements

### Hardware
- 3D printer with serial/USB interface (tested with Creality CR-200B)
- USB cable to connect printer to Raspberry Pi or host machine
- Serial port access (e.g., `/dev/ttyUSB0` on Linux, `/dev/tty.usbserial-*` on macOS)

### Software
- G-code compatible printer firmware (Marlin, RepRap, etc.)
- Serial port permissions for the user running hkcam

## Configuration

### Command Line Flags

Add these flags when starting hkcam to enable printer control:

```bash
hkcam \
  --printer_port=/dev/ttyUSB0 \
  --printer_baudrate=115200 \
  --max_bed_temp=110 \
  --max_nozzle_temp=260
```

#### Available Flags

| Flag | Default | Description |
|------|---------|-------------|
| `printer_port` | `""` (disabled) | Serial port for 3D printer (e.g., `/dev/ttyUSB0`) |
| `printer_baudrate` | `115200` | Serial communication baudrate |
| `max_bed_temp` | `110` | Maximum bed temperature in Celsius |
| `max_nozzle_temp` | `260` | Maximum nozzle temperature in Celsius |

### Finding Your Serial Port

**Linux:**
```bash
ls /dev/ttyUSB* /dev/ttyACM*
# or
dmesg | grep tty
```

**macOS:**
```bash
ls /dev/tty.usb*
```

### Setting Permissions (Linux)

Add your user to the `dialout` group to access serial ports:

```bash
sudo usermod -a -G dialout $USER
# Log out and back in for changes to take effect
```

Or set specific permissions:
```bash
sudo chmod 666 /dev/ttyUSB0
```

## Usage

### Example: Complete Setup

```bash
# Start hkcam with camera and printer control
hkcam \
  --data_dir=/var/lib/hkcam/data \
  --verbose=true \
  --multi_stream=true \
  --printer_port=/dev/ttyUSB0 \
  --printer_baudrate=115200 \
  --max_bed_temp=110 \
  --max_nozzle_temp=260
```

### Example: Raspberry Pi with Service

Edit your service file (e.g., `/etc/sv/hkcam/run` for runit):

```bash
#!/bin/sh -e
exec 2>&1
v4l2-ctl --set-fmt-video=width=1280,height=720,pixelformat=YU12
exec hkcam \
  --data_dir=/var/lib/hkcam/data \
  --verbose=true \
  --multi_stream=true \
  --printer_port=/dev/ttyUSB0 \
  --printer_baudrate=115200
```

### In HomeKit Apps

Once configured, the printer services will appear alongside your camera in HomeKit apps:

1. **Print Control** - Toggle to pause/resume printing
2. **Bed Temperature** - View current temp and set target
3. **Nozzle Temperature** - View current temp and set target
4. **Part Cooling Fan** - Toggle fan on/off

**Note:** These services are only added if `--printer_port` is specified. If the printer connection fails, hkcam will continue to run with just the camera features.

## Supported G-code Commands

The printer controller uses the following standard G-code commands:

- `M105` - Get current temperatures
- `M140 S<temp>` - Set bed target temperature
- `M104 S<temp>` - Set nozzle target temperature
- `M106 S<speed>` - Turn on fan with speed (0-255)
- `M107` - Turn off fan
- `M24` - Resume SD print
- `M25` - Pause SD print
- `M27` - Get SD print status

These commands are compatible with most modern 3D printer firmware including Marlin, RepRap, and others.

## Monitoring

### Temperature Updates
- Temperatures are polled every 5 seconds
- HomeKit is notified immediately when temperatures change
- Heating state automatically updates based on target vs current temperature

### Print Status
- Print status is checked every 5 seconds via SD card status
- HomeKit switch updates automatically when printing starts/stops/pauses

### Fan State
- Fan state is synchronized with HomeKit
- Automatically turns off when print is cancelled (both temps go to 0)

## Troubleshooting

### Printer Not Connecting

1. Check serial port exists:
   ```bash
   ls -l /dev/ttyUSB0
   ```

2. Verify permissions:
   ```bash
   groups  # Should include 'dialout' on Linux
   ```

3. Check baudrate matches your printer (115200 is most common)

4. Enable verbose logging to see connection details:
   ```bash
   hkcam --verbose=true --printer_port=/dev/ttyUSB0
   ```

### Services Not Appearing in HomeKit

1. Make sure printer connected successfully (check logs)
2. Try removing and re-adding the accessory in HomeKit
3. Restart the HomeKit app or device

### Temperature Not Updating

1. Check printer is responding to `M105` commands (enable verbose logging)
2. Verify printer firmware supports temperature reporting
3. Check serial connection is stable

### Printer Busy Errors

The controller automatically retries temperature queries up to 3 times if the printer reports being busy. This is normal during intensive operations.

## Implementation Details

### Architecture

The printer control integration consists of three main components:

1. **`printer/controller.go`** - Low-level serial communication and G-code handling
2. **`printer_control.go`** - HomeKit service management and state synchronization
3. **`cmd/hkcam/main.go`** - Integration with the camera accessory

### Thread Safety

- All state reads/writes are protected with mutexes
- Serial commands are serialized to prevent conflicts
- Callbacks are executed in separate goroutines to prevent blocking

### State Synchronization

- Controller polls printer every 5 seconds for updates
- HomeKit characteristics are updated immediately when state changes
- User commands are sent immediately with optional response waiting

## Testing Without a Printer

To test the implementation without physical hardware, you can use a virtual serial port:

**Linux:**
```bash
socat -d -d pty,raw,echo=0 pty,raw,echo=0
# Use one of the created /dev/pts/X devices
```

**macOS:**
```bash
# Create a virtual serial port pair
socat -d -d pty,raw,echo=0,link=/tmp/ttyV0 pty,raw,echo=0,link=/tmp/ttyV1
# Connect hkcam to /tmp/ttyV0
# Simulate printer on /tmp/ttyV1
```

## Python PoC Comparison

This Go implementation provides the same functionality as the Python PoC (`poc/` directory) but with several advantages:

1. **Integrated with hkcam** - No separate service needed
2. **Better performance** - Native compiled code
3. **Simpler deployment** - Single binary
4. **Native HomeKit** - Uses the same hap library as the camera

The G-code commands, state management, and HomeKit characteristics are functionally identical to the Python version.

## Safety Considerations

⚠️ **Important Safety Notes:**

1. Never leave your 3D printer unattended while printing
2. This integration does not replace proper printer monitoring
3. Always ensure proper printer configuration and safety features
4. Temperature limits are software-enforced but hardware should have its own safety limits
5. The controller does not validate if commands are safe for your specific printer

## Future Enhancements

Potential future improvements:

- [ ] Print progress monitoring
- [ ] Print time estimation
- [ ] Filament runout detection
- [ ] Multi-printer support
- [ ] Custom HomeKit characteristics for print statistics
- [ ] Integration with OctoPrint or similar
- [ ] Temperature graphs via web interface
- [ ] Email/notification alerts for print completion or issues

## Contributing

If you have improvements or support for additional printers, please open an issue or pull request on the [hkcam GitHub repository](https://github.com/brutella/hkcam).

## License

This printer control integration follows the same license as hkcam. See [LICENSE](LICENSE) for details.
