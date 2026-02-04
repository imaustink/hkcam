package hkcam

import (
	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/log"
	"github.com/brutella/hap/service"
	"github.com/brutella/hkcam/light"
)

// Re-export light.Controller as LightController for convenience
type LightController = light.Controller

// NewLightController creates a new LED light controller.
func NewLightController(gpioPin, ledCount int) *LightController {
	return light.NewController(gpioPin, ledCount)
}

// LightControl manages LED light HomeKit services.
type LightControl struct {
	Lightbulb  *service.Lightbulb
	Brightness *characteristic.Brightness
	Hue        *characteristic.Hue
	Saturation *characteristic.Saturation
	controller *light.Controller
}

// NewLightControl creates a new light control with HomeKit lightbulb service.
func NewLightControl(controller *light.Controller) *LightControl {
	lc := &LightControl{
		controller: controller,
	}

	lc.setupLightbulb()

	// Register callback from controller
	controller.SetOnStateUpdate(lc.onStateUpdate)

	return lc
}

// setupLightbulb creates a lightbulb service for LED control.
func (lc *LightControl) setupLightbulb() {
	lc.Lightbulb = service.NewLightbulb()
	lc.Lightbulb.Primary = false

	// Setup On characteristic
	onChar := lc.Lightbulb.On
	onChar.SetValue(false)

	onChar.OnValueRemoteUpdate(func(value bool) {
		log.Info.Printf("Light set to: %v", value)
		if err := lc.controller.SetOn(value); err != nil {
			log.Info.Printf("Failed to set light state: %v", err)
		}
	})

	// Add and setup Brightness characteristic
	lc.Brightness = characteristic.NewBrightness()
	lc.Brightness.SetValue(100)
	lc.Brightness.SetMinValue(0)
	lc.Brightness.SetMaxValue(100)
	lc.Brightness.SetStepValue(1)

	lc.Brightness.OnValueRemoteUpdate(func(value int) {
		log.Info.Printf("Light brightness set to: %d%%", value)
		if err := lc.controller.SetBrightness(value); err != nil {
			log.Info.Printf("Failed to set brightness: %v", err)
		}
	})
	lc.Lightbulb.AddC(lc.Brightness.C)

	// Add and setup Hue characteristic (0-360 degrees)
	lc.Hue = characteristic.NewHue()
	lc.Hue.SetValue(0.0)
	lc.Hue.SetMinValue(0.0)
	lc.Hue.SetMaxValue(360.0)
	lc.Hue.SetStepValue(1.0)

	lc.Hue.OnValueRemoteUpdate(func(value float64) {
		log.Info.Printf("Light hue set to: %.0f°", value)
		// Get current saturation to compute final color
		saturation := lc.Saturation.Value() / 100.0
		if err := lc.updateColor(value, saturation); err != nil {
			log.Info.Printf("Failed to set hue: %v", err)
		}
	})
	lc.Lightbulb.AddC(lc.Hue.C)

	// Add and setup Saturation characteristic (0-100%)
	lc.Saturation = characteristic.NewSaturation()
	lc.Saturation.SetValue(100.0)
	lc.Saturation.SetMinValue(0.0)
	lc.Saturation.SetMaxValue(100.0)
	lc.Saturation.SetStepValue(1.0)

	lc.Saturation.OnValueRemoteUpdate(func(value float64) {
		log.Info.Printf("Light saturation set to: %.0f%%", value)
		// Get current hue to compute final color
		hue := lc.Hue.Value()
		if err := lc.updateColor(hue, value/100.0); err != nil {
			log.Info.Printf("Failed to set saturation: %v", err)
		}
	})
	lc.Lightbulb.AddC(lc.Saturation.C)
}

// updateColor updates the LED color from hue and saturation values.
// When saturation is 0, it produces white.
func (lc *LightControl) updateColor(hue, saturation float64) error {
	rgb := lc.hsvToRGB(hue, saturation, 1.0)
	return lc.controller.SetColor(rgb.R, rgb.G, rgb.B)
}

// hsvToRGB converts HSV to RGB (helper function).
// h: 0-360, s: 0-1, v: 0-1
func (lc *LightControl) hsvToRGB(h, s, v float64) struct{ R, G, B uint8 } {
	if s == 0 {
		val := uint8(v * 255)
		return struct{ R, G, B uint8 }{R: val, G: val, B: val}
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

	return struct{ R, G, B uint8 }{
		R: uint8(r * 255),
		G: uint8(g * 255),
		B: uint8(b * 255),
	}
}

// onStateUpdate is called when the LED state changes.
func (lc *LightControl) onStateUpdate() {
	on, color, brightness := lc.controller.GetState()

	// Update HomeKit characteristics
	lc.Lightbulb.On.SetValue(on)
	lc.Brightness.SetValue(brightness)

	// Convert RGB to HSV for HomeKit
	h, s, _ := lc.rgbToHSV(color.R, color.G, color.B)
	lc.Hue.SetValue(h)
	lc.Saturation.SetValue(s * 100)
}

// rgbToHSV converts RGB to HSV (helper function).
func (lc *LightControl) rgbToHSV(r, g, b uint8) (h, s, v float64) {
	rf := float64(r) / 255.0
	gf := float64(g) / 255.0
	bf := float64(b) / 255.0

	max := rf
	if gf > max {
		max = gf
	}
	if bf > max {
		max = bf
	}

	min := rf
	if gf < min {
		min = gf
	}
	if bf < min {
		min = bf
	}

	v = max
	delta := max - min

	if max != 0 {
		s = delta / max
	} else {
		return 0, 0, 0
	}

	if delta == 0 {
		return 0, 0, v
	}

	if rf == max {
		h = (gf - bf) / delta
	} else if gf == max {
		h = 2 + (bf-rf)/delta
	} else {
		h = 4 + (rf-gf)/delta
	}

	h *= 60
	if h < 0 {
		h += 360
	}

	return h, s, v
}
