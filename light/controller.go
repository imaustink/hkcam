package light

import (
	"fmt"
	"sync"

	"github.com/brutella/hap/log"
	ws2811 "github.com/rpi-ws281x/rpi-ws281x-go"
)

// Controller handles WS2812b LED control via GPIO.
type Controller struct {
	gpioPin      int
	ledCount     int
	brightness   int
	ws           *ws2811.WS2811
	connected    bool
	on           bool
	currentColor RGB

	// Callbacks for state changes
	onStateUpdate func()

	// Threading
	controlLock sync.Mutex
	stateLock   sync.RWMutex
}

// RGB represents an RGB color value.
type RGB struct {
	R uint8
	G uint8
	B uint8
}

// NewController creates a new LED controller.
// gpioPin: GPIO pin number (typically 18 for PWM0)
// ledCount: number of LEDs in the strip
func NewController(gpioPin, ledCount int) *Controller {
	return &Controller{
		gpioPin:      gpioPin,
		ledCount:     ledCount,
		brightness:   255,
		on:           false,
		currentColor: RGB{R: 255, G: 255, B: 255}, // Default to white
	}
}

// Connect initializes the WS2812b LED controller.
func (c *Controller) Connect() error {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()

	if c.connected {
		return nil
	}

	opt := ws2811.DefaultOptions
	opt.Channels[0].Brightness = c.brightness
	opt.Channels[0].LedCount = c.ledCount
	opt.Channels[0].GpioPin = c.gpioPin

	ws, err := ws2811.MakeWS2811(&opt)
	if err != nil {
		return fmt.Errorf("failed to initialize WS2812b: %v", err)
	}

	if err := ws.Init(); err != nil {
		return fmt.Errorf("failed to initialize WS2812b: %v", err)
	}

	c.ws = ws
	c.connected = true

	log.Info.Printf("LED controller connected on GPIO %d with %d LEDs", c.gpioPin, c.ledCount)

	// Initialize to off state
	c.setAllLEDs(0)
	if err := c.ws.Render(); err != nil {
		return fmt.Errorf("failed to render LEDs: %v", err)
	}

	return nil
}

// Disconnect closes the LED controller.
func (c *Controller) Disconnect() error {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()

	if !c.connected {
		return nil
	}

	// Turn off all LEDs before disconnecting
	c.setAllLEDs(0)
	if c.ws != nil {
		c.ws.Render()
		c.ws.Fini()
	}

	c.connected = false
	log.Info.Println("LED controller disconnected")

	return nil
}

// IsConnected returns whether the controller is connected.
func (c *Controller) IsConnected() bool {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.connected
}

// SetOn turns the LEDs on or off.
func (c *Controller) SetOn(on bool) error {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()

	if !c.connected {
		return fmt.Errorf("LED controller not connected")
	}

	c.stateLock.Lock()
	c.on = on
	c.stateLock.Unlock()

	if on {
		// Turn on with current color
		color := c.rgbToUint32(c.currentColor)
		c.setAllLEDs(color)
	} else {
		// Turn off
		c.setAllLEDs(0)
	}

	if err := c.ws.Render(); err != nil {
		return fmt.Errorf("failed to render LEDs: %v", err)
	}

	log.Info.Printf("LEDs turned %v", map[bool]string{true: "on", false: "off"}[on])

	if c.onStateUpdate != nil {
		c.onStateUpdate()
	}

	return nil
}

// SetBrightness sets the LED brightness (0-100).
func (c *Controller) SetBrightness(brightness int) error {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()

	if !c.connected {
		return fmt.Errorf("LED controller not connected")
	}

	if brightness < 0 || brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}

	// Convert 0-100 to 0-255
	c.brightness = int(float64(brightness) * 2.55)
	c.ws.SetBrightness(0, c.brightness)

	// Re-render with new brightness if LEDs are on
	if c.on {
		color := c.rgbToUint32(c.currentColor)
		c.setAllLEDs(color)
		if err := c.ws.Render(); err != nil {
			return fmt.Errorf("failed to render LEDs: %v", err)
		}
	}

	log.Info.Printf("LED brightness set to %d%%", brightness)

	if c.onStateUpdate != nil {
		c.onStateUpdate()
	}

	return nil
}

// SetColor sets the RGB color of the LEDs.
func (c *Controller) SetColor(r, g, b uint8) error {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()

	if !c.connected {
		return fmt.Errorf("LED controller not connected")
	}

	c.stateLock.Lock()
	c.currentColor = RGB{R: r, G: g, B: b}
	c.stateLock.Unlock()

	// Only update if LEDs are on
	if c.on {
		color := c.rgbToUint32(c.currentColor)
		c.setAllLEDs(color)
		if err := c.ws.Render(); err != nil {
			return fmt.Errorf("failed to render LEDs: %v", err)
		}
	}

	log.Info.Printf("LED color set to RGB(%d, %d, %d)", r, g, b)

	if c.onStateUpdate != nil {
		c.onStateUpdate()
	}

	return nil
}

// SetHue sets the hue of the LEDs (0-360 degrees).
// Saturation is assumed to be 100%.
func (c *Controller) SetHue(hue float64) error {
	rgb := c.hsvToRGB(hue, 1.0, 1.0)
	return c.SetColor(rgb.R, rgb.G, rgb.B)
}

// SetSaturation adjusts the saturation while maintaining current hue.
func (c *Controller) SetSaturation(saturation float64) error {
	c.stateLock.RLock()
	currentColor := c.currentColor
	c.stateLock.RUnlock()

	h, _, v := c.rgbToHSV(currentColor)
	rgb := c.hsvToRGB(h, saturation, v)
	return c.SetColor(rgb.R, rgb.G, rgb.B)
}

// GetState returns the current LED state.
func (c *Controller) GetState() (on bool, color RGB, brightness int) {
	c.stateLock.RLock()
	defer c.stateLock.RUnlock()
	return c.on, c.currentColor, int(float64(c.brightness) / 2.55)
}

// SetOnStateUpdate sets the callback for state changes.
func (c *Controller) SetOnStateUpdate(callback func()) {
	c.controlLock.Lock()
	defer c.controlLock.Unlock()
	c.onStateUpdate = callback
}

// setAllLEDs sets all LEDs to the same color.
func (c *Controller) setAllLEDs(color uint32) {
	for i := 0; i < c.ledCount; i++ {
		c.ws.Leds(0)[i] = color
	}
}

// rgbToUint32 converts RGB to uint32 color value for WS2811.
func (c *Controller) rgbToUint32(rgb RGB) uint32 {
	return uint32(rgb.R)<<16 | uint32(rgb.G)<<8 | uint32(rgb.B)
}

// hsvToRGB converts HSV to RGB.
// h: 0-360, s: 0-1, v: 0-1
func (c *Controller) hsvToRGB(h, s, v float64) RGB {
	if s == 0 {
		val := uint8(v * 255)
		return RGB{R: val, G: val, B: val}
	}

	h = h / 60.0
	i := int(h)
	f := h - float64(i)
	p := v * (1 - s)
	q := v * (1 - s*f)
	t := v * (1 - s*(1-f))

	var r, g, b float64
	switch i % 6 {
	case 0:
		r, g, b = v, t, p
	case 1:
		r, g, b = q, v, p
	case 2:
		r, g, b = p, v, t
	case 3:
		r, g, b = p, q, v
	case 4:
		r, g, b = t, p, v
	case 5:
		r, g, b = v, p, q
	}

	return RGB{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
	}
}

// rgbToHSV converts RGB to HSV.
func (c *Controller) rgbToHSV(rgb RGB) (h, s, v float64) {
	r := float64(rgb.R) / 255.0
	g := float64(rgb.G) / 255.0
	b := float64(rgb.B) / 255.0

	max := r
	if g > max {
		max = g
	}
	if b > max {
		max = b
	}

	min := r
	if g < min {
		min = g
	}
	if b < min {
		min = b
	}

	v = max
	delta := max - min

	if max != 0 {
		s = delta / max
	} else {
		s = 0
		h = 0
		return
	}

	if delta == 0 {
		h = 0
		return
	}

	if r == max {
		h = (g - b) / delta
	} else if g == max {
		h = 2 + (b-r)/delta
	} else {
		h = 4 + (r-g)/delta
	}

	h *= 60
	if h < 0 {
		h += 360
	}

	return
}
