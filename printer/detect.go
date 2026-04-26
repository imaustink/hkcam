package printer

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/brutella/hap/log"
)

// DetectSerialPort attempts to automatically detect a connected 3D printer's serial port.
// It returns the first available /dev/ttyUSB* or /dev/ttyACM* device, or an error if none found.
func DetectSerialPort() (string, error) {
	// Try USB serial devices (common for CH340/CH341 and FTDI adapters)
	devices, err := filepath.Glob("/dev/ttyUSB*")
	if err == nil && len(devices) > 0 {
		log.Info.Printf("Found USB serial device: %s", devices[0])
		return devices[0], nil
	}

	// Try ACM serial devices (common for Arduino-based printers)
	devices, err = filepath.Glob("/dev/ttyACM*")
	if err == nil && len(devices) > 0 {
		log.Info.Printf("Found ACM serial device: %s", devices[0])
		return devices[0], nil
	}

	return "", fmt.Errorf("no printer serial port found")
}

// FindBestSerialPort tries to find the printer serial port.
// If the specified port exists, it returns that. Otherwise, it attempts auto-detection.
func FindBestSerialPort(preferredPort string) (string, error) {
	// First try the preferred port if specified
	if preferredPort != "" {
		// Check if device file exists (don't try to read it)
		if _, err := os.Stat(preferredPort); err == nil {
			log.Info.Printf("Using specified serial port: %s", preferredPort)
			return preferredPort, nil
		}
		log.Info.Printf("Specified port %s not available, attempting auto-detection", preferredPort)
	}

	// Auto-detect
	return DetectSerialPort()
}
