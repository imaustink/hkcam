# GitHub Copilot Instructions for hkcam

## Project Overview

**hkcam** is a HomeKit-enabled security camera project for Raspberry Pi that integrates:
- Video streaming via ffmpeg with HomeKit RTP support
- 3D printer control via serial G-code communication
- WS2812b LED light control
- Web interface for snapshots and status monitoring

## Architecture

### Key Components

1. **Camera Streaming** (`ffmpeg/`)
   - Uses ffmpeg for H.264 video encoding
   - Supports loopback devices for multi-viewer access
   - RTP/SRTP streaming to HomeKit clients
   - Automatic stale connection cleanup (2-minute timeout)

2. **Printer Control** (`printer/`)
   - Serial communication with 3D printers (G-code)
   - Auto-detection of serial ports (`/dev/ttyUSB*`, `/dev/ttyACM*`)
   - Automatic reconnection on device disconnection/port changes
   - Temperature monitoring (bed & nozzle)
   - Print status tracking (printing/paused/idle)
   - Fan control

3. **LED Control** (`light/`)
   - WS2812b RGB LED strip control
   - Uses rpi-ws281x-go library (requires native build on device)

4. **Web API** (`api/`)
   - RESTful endpoints for system info, snapshots, printer status
   - JSON responses with standard error handling

5. **Web Interface** (`html/`)
   - Go templates with Bootstrap UI
   - Real-time status polling (cameras, printer)
   - Snapshot viewing and management

## Development Workflow

### Building

**⚠️ CRITICAL: Cross-compilation fails due to native C dependencies (rpi-ws281x-go)**

Always build on the target Raspberry Pi device, not locally:

```bash
# Sync code to device
rsync -avz --exclude='.git' --exclude='build' --exclude='db' . printcam:/tmp/hkcam-update/

# SSH to device and build
ssh printcam
cd /tmp/hkcam-update
make build
```

### Deployment

```bash
# Stop service
sudo systemctl stop hkcam

# Replace binary
sudo cp /tmp/hkcam-new /opt/hkcam/hkcam
sudo chmod +x /opt/hkcam/hkcam

# Restart service
sudo systemctl start hkcam

# Monitor logs
sudo journalctl -u hkcam -f
```

### Resetting HomeKit Pairing

To reset the HomeKit pairing and re-pair the device (useful when pairing issues occur or switching Home hubs):

```bash
# Stop service
sudo systemctl stop hkcam

# Remove pairing database
sudo rm -rf /opt/hkcam/db/*

# Restart service
sudo systemctl start hkcam

# Check status
sudo systemctl status hkcam
```

After reset:
- Device will appear as a new unpaired accessory in HomeKit
- Use PIN `00102003` (or configured PIN) to pair
- All HomeKit settings and automations will need to be recreated

**Database Location:** `/opt/hkcam/db/` (controlled by `--data_dir` flag, default is `db` relative to working directory)

### Debugging on Device

```bash
# View recent logs
sudo journalctl -u hkcam -n 100 --no-pager

# Check serial devices
ls -la /dev/ttyUSB* /dev/ttyACM*

# Check video devices
ls -la /dev/video*

# View process and arguments
ps aux | grep hkcam | grep -v grep

# Check USB device changes
dmesg | grep -i "ttyUSB\|serial" | tail -30

# Check for open serial port locks
lsof /dev/ttyUSB0
```

## Code Patterns & Conventions

### Serial Port Handling

**Always use auto-detection and reconnection:**

```go
// Use FindBestSerialPort instead of hardcoded paths
port, err := printer.FindBestSerialPort("/dev/ttyUSB0")

// Implement reconnection logic in monitoring loops
if err != nil {
    failureCount++
    if failureCount >= maxFailures {
        c.attemptReconnect()
    }
}
```

### State Management

**Use mutex locks for concurrent access:**

```go
type Controller struct {
    stateLock   sync.RWMutex  // Use RWMutex for read-heavy operations
    commandLock sync.Mutex     // Use Mutex for command serialization
}

// Read operations
func (c *Controller) GetTemperature() (current, target float64) {
    c.stateLock.RLock()
    defer c.stateLock.RUnlock()
    return c.tempCurrent, c.tempTarget
}

// Write operations
func (c *Controller) SetTemperature(temp float64) error {
    c.stateLock.Lock()
    defer c.stateLock.Unlock()
    c.tempTarget = temp
    return nil
}
```

### Callback Pattern

**Use callbacks for state change notifications:**

```go
// Register callbacks in setup
controller.SetOnTempUpdate(pc.onTempUpdate)
controller.SetOnPrintStatusUpdate(pc.onPrintStatusUpdate)

// Trigger callbacks after state changes
if changed && c.onTempUpdate != nil {
    c.onTempUpdate()  // Or use goroutine: go c.onTempUpdate()
}
```

### Camera Snapshots

**Use device-native framerate for snapshots:**

```go
// ✅ Correct - no framerate specified, uses device settings
arg := fmt.Sprintf("-f %s -i %s -vf scale=%d:-2 -frames:v 1 %s", 
    inputDevice, inputFilename, width, filePath)

// ❌ Wrong - hardcoded framerate causes hang if device uses different rate
arg := fmt.Sprintf("-f %s -framerate 30 -i %s -vf scale=%d:-2 -frames:v 1 %s", 
    inputDevice, inputFilename, width, filePath)
```

For streaming (HomeKit RTP), framerate comes from client's requested parameters via `attr.Framerate`.

### HomeKit RTP Streaming

**Always use named struct fields for RTP types:**

```go
// ✅ Correct - named fields
rtp.StreamingStatus{Status: rtp.StreamingStatusAvailable}

// ❌ Wrong - positional fields
rtp.StreamingStatus{rtp.StreamingStatusAvailable}
```

### Stream Lifecycle Management

**Track activity and clean up stale streams:**

```go
type stream struct {
    lastActive time.Time
}

func (s *stream) updateActivity() {
    s.lastActive = time.Now()
}

func (s *stream) isStale(timeout time.Duration) bool {
    return time.Since(s.lastActive) > timeout
}
```

### API Design

**Standard JSON response format:**

```go
type Response struct {
    Data ResponseData `json:"data"`
}

type ResponseData struct {
    Success bool   `json:"success"`
    // ... other fields
}

func Handler(w http.ResponseWriter, r *http.Request) {
    resp := Response{Data: ResponseData{Success: true}}
    if err := WriteJSON(w, r, resp); err != nil {
        log.Info.Println("responding failed", err)
    }
}
```

### Frontend Polling

**Use setInterval for status updates:**

```javascript
// Poll every 3 seconds for status
setInterval(function() {
    updatePrinterStatus()
}, 3000)

function updatePrinterStatus() {
    var xhttp = new APIRequest("GET", `/api/printer/status`);
    xhttp.onreadystatechange = function() {
        if (xhttp.readyState == xhttp.DONE && xhttp.status == 200) {
            var resp = JSON.parse(xhttp.responseText)
            // Update UI
        }
    };
    xhttp.send(null);
}
```

## Common Issues & Solutions

### Issue: Serial Port I/O Errors

**Symptom:** `write /dev/ttyUSB0: input/output error`

**Solution:** 
- USB device was unplugged/reset and came back as different device (ttyUSB0 → ttyUSB1)
- Auto-detection finds new port automatically
- Reconnection logic triggers after 3 failures

### Issue: Camera Not Responding / Snapshot Timeouts

**Symptom:** Snapshot API timeouts, no video stream, connections hang

**Root Cause:** ffmpeg snapshot command had hardcoded `-framerate 30` but camera configured for 5 fps

**Solution:**
- Removed hardcoded framerate from snapshot command in `ffmpeg/snapshot.go`
- Let ffmpeg use device's native framerate instead of forcing specific rate
- Mismatch between requested and actual framerate caused ffmpeg to hang
- Stale stream cleanup runs every 30 seconds
- Streams inactive > 2 minutes are automatically terminated
- Loopback device released when no active streams

### Issue: Status Not Updating in UI

**Symptom:** Temperature/fan/print status shows stale data

**Solution:**
- Ensure API endpoint returns current data
- Frontend polling interval should be 3-5 seconds
- Check browser console for API errors

### Issue: Cross-Compilation Fails

**Symptom:** `ws2811.h file not found` or `undefined: WS2811`

**Solution:**
- **Never cross-compile from macOS/other systems**
- Always build directly on Raspberry Pi
- Native C libraries (rpi-ws281x) require device-native compilation

## Testing Checklist

Before deploying changes:

- [ ] Build completes without errors on device
- [ ] Serial port auto-detection works (unplug/replug USB)
- [ ] Temperature updates appear in logs every 5 seconds
- [ ] Web UI shows current printer status
- [ ] Camera stream starts successfully
- [ ] Multiple camera viewers can connect
- [ ] Stale streams clean up after inactivity
- [ ] Service survives restart: `sudo sv restart hkcam`

## File Structure Reference

```
hkcam/
├── cmd/hkcam/          # Main application entry point
├── api/                # REST API handlers
├── printer/            # 3D printer control
│   ├── controller.go   # Serial communication & monitoring
│   └── detect.go       # Auto-detection logic
├── ffmpeg/             # Video streaming
│   ├── ffmpeg.go       # Stream management
│   ├── stream.go       # Individual stream handling
│   └── loopback.go     # Multi-viewer support
├── light/              # LED control
├── html/               # Web interface
│   └── tmpl/           # Go templates
├── static/             # CSS/JS assets
└── api/                # API layer
    └── printer.go      # Printer status endpoint
```

## Key Dependencies

- `github.com/brutella/hap` - HomeKit Accessory Protocol
- `github.com/tarm/serial` - Serial port communication
- `github.com/rpi-ws281x/rpi-ws281x-go` - LED control (native only)
- `github.com/go-chi/chi` - HTTP router
- FFmpeg (system binary) - Video encoding/streaming

## Performance Considerations

- Monitor loop runs every 5 seconds (balance between responsiveness and load)
- Stream cleanup runs every 30 seconds (prevents resource leaks)
- Frontend polling at 3 seconds (good UX without overloading)
- Command locks prevent parallel G-code commands (printer safety)
- Read locks allow concurrent temperature reads (performance)

## Security Notes

- HomeKit pairing requires 8-digit PIN (default: 00102003)
- Web interface accessible on port 8080 (configure firewall as needed)
- Serial port requires appropriate permissions (user in dialout group)
- No authentication on API endpoints (assumes trusted local network)
