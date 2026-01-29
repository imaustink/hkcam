package printer

import (
	"bufio"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/brutella/hap/log"
	"github.com/tarm/serial"
)

// Controller handles serial communication with a 3D printer using G-code commands.
type Controller struct {
	port      string
	baudrate  int
	timeout   time.Duration
	serial    *serial.Port
	connected bool

	// State variables
	bedTempCurrent    float64
	bedTempTarget     float64
	nozzleTempCurrent float64
	nozzleTempTarget  float64
	isPrinting        bool
	isPaused          bool
	fanOn             bool

	// Callbacks for state changes
	onTempUpdate        func()
	onPrintStatusUpdate func()
	onFanUpdate         func()

	// Threading
	stopChan    chan struct{}
	commandLock sync.Mutex
	stateLock   sync.RWMutex
}

// NewController creates a new printer controller.
func NewController(port string, baudrate int, timeout time.Duration) *Controller {
	return &Controller{
		port:     port,
		baudrate: baudrate,
		timeout:  timeout,
		stopChan: make(chan struct{}),
	}
}

// Connect establishes connection to the printer.
func (c *Controller) Connect() error {
	config := &serial.Config{
		Name:        c.port,
		Baud:        c.baudrate,
		ReadTimeout: c.timeout,
	}

	s, err := serial.OpenPort(config)
	if err != nil {
		return fmt.Errorf("failed to open serial port: %w", err)
	}

	c.serial = s
	time.Sleep(2 * time.Second) // Wait for printer to initialize

	// Clear any startup messages
	c.serial.Flush()

	c.connected = true
	log.Info.Printf("Connected to printer on %s", c.port)

	// Initialize fan to on state
	log.Info.Println("Initializing fan state to on")
	c.sendCommand("M106 S255", false) // Turn fan on at max speed
	c.stateLock.Lock()
	c.fanOn = true
	c.stateLock.Unlock()
	time.Sleep(500 * time.Millisecond)

	// Start monitoring thread
	go c.monitorLoop()

	return nil
}

// Disconnect closes the connection to the printer.
func (c *Controller) Disconnect() error {
	if c.stopChan != nil {
		close(c.stopChan)
	}

	if c.serial != nil {
		return c.serial.Close()
	}

	c.connected = false
	log.Info.Println("Disconnected from printer")
	return nil
}

// sendCommand sends a G-code command to the printer.
func (c *Controller) sendCommand(command string, waitForResponse bool) (string, error) {
	if !c.connected || c.serial == nil {
		return "", fmt.Errorf("not connected to printer")
	}

	c.commandLock.Lock()
	defer c.commandLock.Unlock()

	// Flush input buffer before sending
	c.serial.Flush()

	_, err := c.serial.Write([]byte(command + "\n"))
	if err != nil {
		return "", fmt.Errorf("failed to send command: %w", err)
	}

	log.Debug.Printf("Sent: %s", command)

	if !waitForResponse {
		return "", nil
	}

	// Read response
	scanner := bufio.NewScanner(c.serial)
	var response strings.Builder
	maxAttempts := 15
	attempts := 0
	gotOk := false

	for attempts < maxAttempts && scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			response.WriteString(line)
			response.WriteString("\n")
			log.Debug.Printf("Received: %s", line)

			lowerLine := strings.ToLower(line)
			if strings.Contains(lowerLine, "ok") || strings.Contains(lowerLine, "error") {
				gotOk = true
				attempts++
				continue
			}
		} else if gotOk {
			break
		}
		attempts++
	}

	return response.String(), nil
}

// monitorLoop runs in the background to monitor printer status.
func (c *Controller) monitorLoop() {
	// Initial status check
	time.Sleep(3 * time.Second)
	c.checkPrintStatus()

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.updateTemperatures()
			c.checkPrintStatus()
		}
	}
}

// checkPrintStatus checks if the printer is currently printing.
func (c *Controller) checkPrintStatus() {
	response, err := c.sendCommand("M27", true) // Get SD print status
	if err != nil {
		log.Info.Printf("Error checking print status: %v", err)
		return
	}

	c.stateLock.Lock()
	wasActive := c.isPrinting && !c.isPaused

	if strings.Contains(response, "SD printing") && !strings.Contains(response, "Not SD printing") {
		c.isPrinting = true
		c.isPaused = false
	} else if c.isPrinting {
		c.isPrinting = false
		c.isPaused = false
	}

	isActive := c.isPrinting && !c.isPaused
	c.stateLock.Unlock()

	if wasActive != isActive && c.onPrintStatusUpdate != nil {
		c.onPrintStatusUpdate()
	}
}

// updateTemperatures queries and updates temperature readings.
func (c *Controller) updateTemperatures() {
	// Retry up to 3 times if printer is busy
	var response string
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		response, err = c.sendCommand("M105", true) // Get temperature
		if err != nil {
			log.Info.Printf("Error getting temperature: %v", err)
			return
		}

		if strings.Contains(strings.ToLower(response), "busy") {
			log.Info.Printf("Printer busy, retrying temperature query (attempt %d/3)", attempt+1)
			time.Sleep(1 * time.Second)
			continue
		}
		break
	}

	// Parse temperature response: T:20.5 /0.0 B:19.8 /0.0
	tempPattern := regexp.MustCompile(`T:(\d+\.?\d*)\s*/(\d+\.?\d*)\s*B:(\d+\.?\d*)\s*/(\d+\.?\d*)`)
	match := tempPattern.FindStringSubmatch(response)

	if match != nil && len(match) == 5 {
		nozzleCurrent, _ := strconv.ParseFloat(match[1], 64)
		nozzleTarget, _ := strconv.ParseFloat(match[2], 64)
		bedCurrent, _ := strconv.ParseFloat(match[3], 64)
		bedTarget, _ := strconv.ParseFloat(match[4], 64)

		c.stateLock.Lock()
		changed := nozzleCurrent != c.nozzleTempCurrent ||
			nozzleTarget != c.nozzleTempTarget ||
			bedCurrent != c.bedTempCurrent ||
			bedTarget != c.bedTempTarget

		c.nozzleTempCurrent = nozzleCurrent
		c.nozzleTempTarget = nozzleTarget
		c.bedTempCurrent = bedCurrent
		c.bedTempTarget = bedTarget

		// Auto-disable fan if both targets are 0 (print cancelled)
		if nozzleTarget == 0 && bedTarget == 0 && c.fanOn {
			log.Info.Println("Temps set to 0 - print cancelled, turning off fan")
			c.fanOn = false
			if c.onFanUpdate != nil {
				go c.onFanUpdate()
			}
		}

		c.stateLock.Unlock()

		if changed && c.onTempUpdate != nil {
			c.onTempUpdate()
		}
	} else {
		log.Info.Printf("Failed to parse temperature from response: %s", response)
	}
}

// SetBedTemperature sets the bed target temperature.
func (c *Controller) SetBedTemperature(temp float64) error {
	log.Info.Printf("Setting bed temperature to %.1f°C", temp)
	_, err := c.sendCommand(fmt.Sprintf("M140 S%.1f", temp), false)
	if err == nil {
		c.stateLock.Lock()
		c.bedTempTarget = temp
		c.stateLock.Unlock()
	}
	return err
}

// SetNozzleTemperature sets the nozzle target temperature.
func (c *Controller) SetNozzleTemperature(temp float64) error {
	log.Info.Printf("Setting nozzle temperature to %.1f°C", temp)
	_, err := c.sendCommand(fmt.Sprintf("M104 S%.1f", temp), false)
	if err == nil {
		c.stateLock.Lock()
		c.nozzleTempTarget = temp
		c.stateLock.Unlock()
	}
	return err
}

// PausePrint pauses the current print.
func (c *Controller) PausePrint() error {
	log.Info.Println("Pausing print")
	_, err := c.sendCommand("M25", true) // Pause SD print
	if err == nil {
		c.stateLock.Lock()
		c.isPaused = true
		c.isPrinting = true
		c.stateLock.Unlock()

		if c.onPrintStatusUpdate != nil {
			c.onPrintStatusUpdate()
		}
	}
	return err
}

// ResumePrint resumes a paused print.
func (c *Controller) ResumePrint() error {
	log.Info.Println("Resuming print")
	_, err := c.sendCommand("M24", true) // Resume SD print
	if err == nil {
		c.stateLock.Lock()
		c.isPaused = false
		c.isPrinting = true
		c.stateLock.Unlock()

		if c.onPrintStatusUpdate != nil {
			c.onPrintStatusUpdate()
		}
	}
	return err
}

// SetFan controls the part cooling fan.
func (c *Controller) SetFan(on bool) error {
	log.Info.Printf("Setting fan %s", map[bool]string{true: "on", false: "off"}[on])

	var cmd string
	if on {
		cmd = "M106 S255" // Turn on fan at max speed
	} else {
		cmd = "M107" // Turn off fan
	}

	_, err := c.sendCommand(cmd, false)
	if err == nil {
		c.stateLock.Lock()
		c.fanOn = on
		c.stateLock.Unlock()
	}
	return err
}

// GetBedTemperature returns the current and target bed temperatures.
func (c *Controller) GetBedTemperature() (current, target float64) {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.bedTempCurrent, c.bedTempTarget
}

// GetNozzleTemperature returns the current and target nozzle temperatures.
func (c *Controller) GetNozzleTemperature() (current, target float64) {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.nozzleTempCurrent, c.nozzleTempTarget
}

// GetPrintStatus returns the current print status.
func (c *Controller) GetPrintStatus() (isPrinting, isPaused, active bool) {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.isPrinting, c.isPaused, c.isPrinting && !c.isPaused
}

// GetFanState returns whether the fan is on.
func (c *Controller) GetFanState() bool {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.fanOn
}

// SetOnTempUpdate sets the callback for temperature updates.
func (c *Controller) SetOnTempUpdate(callback func()) {
	c.onTempUpdate = callback
}

// SetOnPrintStatusUpdate sets the callback for print status updates.
func (c *Controller) SetOnPrintStatusUpdate(callback func()) {
	c.onPrintStatusUpdate = callback
}

// SetOnFanUpdate sets the callback for fan state updates.
func (c *Controller) SetOnFanUpdate(callback func()) {
	c.onFanUpdate = callback
}
