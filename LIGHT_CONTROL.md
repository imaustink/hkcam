# LED Light Control

This module provides HomeKit control for WS2812b LED strips on Raspberry Pi.

## Hardware Setup

### GPIO Pin

The default GPIO pin is **GPIO 18 (Physical Pin 12)** on the Raspberry Pi. This pin supports PWM0 which is ideal for WS2812b LEDs.

### Wiring

Connect your WS2812b LED strip to the Raspberry Pi:

- **LED Data In** → GPIO 18 (Pin 12)
- **LED +5V** → External 5V power supply
- **LED GND** → Common ground with Raspberry Pi and power supply

⚠️ **Important**: 
- WS2812b LEDs require 5V power. Do not power more than a few LEDs directly from the Raspberry Pi's 5V pin.
- Use an external 5V power supply for LED strips with more than 10 LEDs.
- Always connect grounds together (Raspberry Pi GND, LED strip GND, and power supply GND).

### Alternative GPIO Pins

If GPIO 18 is unavailable, you can use:
- **GPIO 12 (Physical Pin 32)** - PWM0 alternative
- **GPIO 13 (Physical Pin 33)** - PWM1
- **GPIO 19 (Physical Pin 35)** - PWM1 alternative

Specify the pin using the `--light_gpio_pin` flag.

## Software Requirements

### Install rpi-ws281x library

On Raspberry Pi, install the required library:

```bash
# Install dependencies
sudo apt-get update
sudo apt-get install -y build-essential git scons swig

# Install rpi_ws281x library
cd /tmp
git clone https://github.com/jgarff/rpi_ws281x.git
cd rpi_ws281x
scons
sudo scons install
```

### Go Module

The project uses the `github.com/rpi-ws281x/rpi-ws281x-go` Go wrapper. Install it with:

```bash
go get github.com/rpi-ws281x/rpi-ws281x-go
```

## Usage

### Enable LED Control

Start hkcam with LED control enabled:

```bash
sudo ./hkcam --enable_lights --led_count 30
```

**Note**: Root privileges (`sudo`) are required for GPIO access on Raspberry Pi.

### Command Line Options

- `--enable_lights` - Enable WS2812b LED control
- `--light_gpio_pin` - GPIO pin number (default: 18)
- `--led_count` - Number of LEDs in your strip (default: 30)

### Example Commands

```bash
# 30 LEDs on default GPIO 18
sudo ./hkcam --enable_lights --led_count 30

# 60 LEDs on GPIO 12
sudo ./hkcam --enable_lights --light_gpio_pin 12 --led_count 60

# Enable with camera and printer
sudo ./hkcam --enable_lights --led_count 30 --printer_port /dev/ttyUSB0
```

## HomeKit Features

Once enabled, the LED strip appears as a **Lightbulb** in HomeKit with full control:

- **On/Off** - Turn lights on or off
- **Brightness** - 0-100% brightness control
- **Color** - Full RGB color selection with hue and saturation
- **Siri** - "Hey Siri, turn on the camera lights"
- **Automation** - Trigger lights based on time, location, or other HomeKit events

## Troubleshooting

### Permission Denied

If you get permission errors:
```bash
# Run with sudo
sudo ./hkcam --enable_lights
```

### LEDs Don't Light Up

1. Check wiring connections
2. Verify GPIO pin number is correct
3. Ensure LED strip is getting proper 5V power
4. Check that the LED count matches your strip

### Flickering or Wrong Colors

1. Ensure stable power supply
2. Add a 470Ω resistor between GPIO and LED data line
3. Add a 1000µF capacitor across the LED strip power supply
4. Keep data wire short (< 6 inches) or use shielded cable

### Library Not Found

If you get `rpi_ws281x` library errors:
```bash
# Reinstall the library
cd /tmp/rpi_ws281x
sudo scons install

# Update library cache
sudo ldconfig
```

## Technical Details

- **Protocol**: WS2812b (NeoPixel compatible)
- **Data Rate**: 800 kHz
- **Color Order**: RGB (Red-Green-Blue)
- **PWM Channel**: 0 (GPIO 18) or 1 (GPIO 13/19)
- **Brightness**: Hardware PWM control for flicker-free dimming
- **Color Space**: HSV internally, RGB to LEDs

## Safety Notes

⚠️ Always:
- Use an external power supply for LED strips
- Connect all grounds together
- Don't exceed the current rating of your power supply
- Consider adding a fuse in the power line
- Test with a small number of LEDs first

## Resources

- [WS2812b Datasheet](https://cdn-shop.adafruit.com/datasheets/WS2812B.pdf)
- [rpi_ws281x Library](https://github.com/jgarff/rpi_ws281x)
- [Raspberry Pi GPIO Guide](https://www.raspberrypi.com/documentation/computers/raspberry-pi.html)
