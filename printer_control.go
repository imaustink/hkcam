package hkcam

import (
	"time"

	"github.com/brutella/hap/characteristic"
	"github.com/brutella/hap/log"
	"github.com/brutella/hap/service"
	"github.com/brutella/hkcam/printer"
)

// Re-export printer.Controller as PrinterController for convenience
type PrinterController = printer.Controller

// NewPrinterController creates a new printer controller.
func NewPrinterController(port string, baudrate int, timeout time.Duration) *PrinterController {
	return printer.NewController(port, baudrate, timeout)
}

// PrinterControl manages printer-related HomeKit services.
type PrinterControl struct {
	PrintSwitch      *service.Switch
	BedThermostat    *service.Thermostat
	NozzleThermostat *service.Thermostat
	Fan              *service.Fan
	controller       *printer.Controller
	maxBedTemp       float64
	maxNozzleTemp    float64
}

// NewPrinterControl creates a new printer control with all services.
func NewPrinterControl(controller *printer.Controller, maxBedTemp, maxNozzleTemp float64) *PrinterControl {
	pc := &PrinterControl{
		controller:    controller,
		maxBedTemp:    maxBedTemp,
		maxNozzleTemp: maxNozzleTemp,
	}

	pc.setupPrintSwitch()
	pc.setupBedThermostat()
	pc.setupNozzleThermostat()
	pc.setupFan()

	// Register callbacks from controller
	controller.SetOnTempUpdate(pc.onTempUpdate)
	controller.SetOnPrintStatusUpdate(pc.onPrintStatusUpdate)
	controller.SetOnFanUpdate(pc.onFanUpdate)

	return pc
}

// setupPrintSwitch creates a switch service for print pause/resume control.
func (pc *PrinterControl) setupPrintSwitch() {
	pc.PrintSwitch = service.NewSwitch()
	pc.PrintSwitch.Primary = false

	// Setup On characteristic for print control
	onChar := pc.PrintSwitch.On
	onChar.SetValue(false)

	onChar.OnValueRemoteUpdate(func(value bool) {
		log.Info.Printf("Print control set to: %v", value)
		if value {
			// ON - resume printing
			if err := pc.controller.ResumePrint(); err != nil {
				log.Info.Printf("Failed to resume print: %v", err)
			}
		} else {
			// OFF - pause printing
			if err := pc.controller.PausePrint(); err != nil {
				log.Info.Printf("Failed to pause print: %v", err)
			}
		}
	})
}

// setupBedThermostat creates a thermostat service for bed temperature control.
func (pc *PrinterControl) setupBedThermostat() {
	pc.BedThermostat = service.NewThermostat()
	pc.BedThermostat.Primary = false

	// Current temperature (read-only)
	currentTempChar := pc.BedThermostat.CurrentTemperature
	currentTempChar.SetValue(0)

	// Target temperature
	targetTempChar := pc.BedThermostat.TargetTemperature
	targetTempChar.SetValue(0)
	targetTempChar.SetMinValue(0)
	targetTempChar.SetMaxValue(pc.maxBedTemp)
	targetTempChar.SetStepValue(1)

	targetTempChar.OnValueRemoteUpdate(func(value float64) {
		log.Info.Printf("Bed temperature set to: %.1f°C", value)
		if err := pc.controller.SetBedTemperature(value); err != nil {
			log.Info.Printf("Failed to set bed temperature: %v", err)
		}
	})

	// Heating/Cooling states
	currentStateChar := pc.BedThermostat.CurrentHeatingCoolingState
	currentStateChar.SetValue(characteristic.CurrentHeatingCoolingStateOff)

	// Target heating/cooling state
	targetStateChar := pc.BedThermostat.TargetHeatingCoolingState
	targetStateChar.SetValue(characteristic.TargetHeatingCoolingStateOff)
	targetStateChar.SetMinValue(characteristic.TargetHeatingCoolingStateOff)
	targetStateChar.SetMaxValue(characteristic.TargetHeatingCoolingStateHeat)

	targetStateChar.OnValueRemoteUpdate(func(value int) {
		log.Info.Printf("Bed heating mode set to: %d", value)
		if value == characteristic.TargetHeatingCoolingStateOff {
			// Off - set temperature to 0
			if err := pc.controller.SetBedTemperature(0); err != nil {
				log.Info.Printf("Failed to turn off bed heater: %v", err)
			}
		} else if value == characteristic.TargetHeatingCoolingStateHeat {
			// Heat - set to default or keep current target
			_, currentTarget := pc.controller.GetBedTemperature()
			if currentTarget == 0 {
				// If currently off, set to a default heating temp
				if err := pc.controller.SetBedTemperature(60); err != nil {
					log.Info.Printf("Failed to set bed temperature: %v", err)
				}
			}
		}
	})

	// Temperature display units (Celsius only)
	unitsChar := pc.BedThermostat.TemperatureDisplayUnits
	unitsChar.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
}

// setupNozzleThermostat creates a thermostat service for nozzle temperature control.
func (pc *PrinterControl) setupNozzleThermostat() {
	pc.NozzleThermostat = service.NewThermostat()
	pc.NozzleThermostat.Primary = false

	// Current temperature (read-only)
	currentTempChar := pc.NozzleThermostat.CurrentTemperature
	currentTempChar.SetValue(0)

	// Target temperature
	targetTempChar := pc.NozzleThermostat.TargetTemperature
	targetTempChar.SetValue(0)
	targetTempChar.SetMinValue(0)
	targetTempChar.SetMaxValue(pc.maxNozzleTemp)
	targetTempChar.SetStepValue(1)

	targetTempChar.OnValueRemoteUpdate(func(value float64) {
		log.Info.Printf("Nozzle temperature set to: %.1f°C", value)
		if err := pc.controller.SetNozzleTemperature(value); err != nil {
			log.Info.Printf("Failed to set nozzle temperature: %v", err)
		}
	})

	// Heating/Cooling states
	currentStateChar := pc.NozzleThermostat.CurrentHeatingCoolingState
	currentStateChar.SetValue(characteristic.CurrentHeatingCoolingStateOff)

	// Target heating/cooling state
	targetStateChar := pc.NozzleThermostat.TargetHeatingCoolingState
	targetStateChar.SetValue(characteristic.TargetHeatingCoolingStateOff)
	targetStateChar.SetMinValue(characteristic.TargetHeatingCoolingStateOff)
	targetStateChar.SetMaxValue(characteristic.TargetHeatingCoolingStateHeat)

	targetStateChar.OnValueRemoteUpdate(func(value int) {
		log.Info.Printf("Nozzle heating mode set to: %d", value)
		if value == characteristic.TargetHeatingCoolingStateOff {
			// Off - set temperature to 0
			if err := pc.controller.SetNozzleTemperature(0); err != nil {
				log.Info.Printf("Failed to turn off nozzle heater: %v", err)
			}
		} else if value == characteristic.TargetHeatingCoolingStateHeat {
			// Heat - set to default or keep current target
			_, currentTarget := pc.controller.GetNozzleTemperature()
			if currentTarget == 0 {
				// If currently off, set to a default heating temp
				if err := pc.controller.SetNozzleTemperature(200); err != nil {
					log.Info.Printf("Failed to set nozzle temperature: %v", err)
				}
			}
		}
	})

	// Temperature display units (Celsius only)
	unitsChar := pc.NozzleThermostat.TemperatureDisplayUnits
	unitsChar.SetValue(characteristic.TemperatureDisplayUnitsCelsius)
}

// setupFan creates a fan service for part cooling fan control.
func (pc *PrinterControl) setupFan() {
	pc.Fan = service.NewFan()
	pc.Fan.Primary = false

	// Setup On characteristic for fan control
	onChar := pc.Fan.On
	onChar.SetValue(true) // Default to on

	onChar.OnValueRemoteUpdate(func(value bool) {
		log.Info.Printf("Fan set to: %v", value)
		if err := pc.controller.SetFan(value); err != nil {
			log.Info.Printf("Failed to set fan: %v", err)
		}
	})
}

// onTempUpdate is called when printer temperatures are updated.
func (pc *PrinterControl) onTempUpdate() {
	// Update bed temperature
	bedCurrent, bedTarget := pc.controller.GetBedTemperature()

	pc.BedThermostat.CurrentTemperature.SetValue(bedCurrent)
	pc.BedThermostat.TargetTemperature.SetValue(bedTarget)

	// Update heating state: heating only if target > 0 AND current < target
	var bedState int
	if bedTarget > 0 && bedCurrent < bedTarget-1 {
		bedState = characteristic.CurrentHeatingCoolingStateHeat
	} else {
		bedState = characteristic.CurrentHeatingCoolingStateOff
	}
	pc.BedThermostat.CurrentHeatingCoolingState.SetValue(bedState)

	// Update target heating state
	var bedTargetState int
	if bedTarget > 0 {
		bedTargetState = characteristic.TargetHeatingCoolingStateHeat
	} else {
		bedTargetState = characteristic.TargetHeatingCoolingStateOff
	}
	pc.BedThermostat.TargetHeatingCoolingState.SetValue(bedTargetState)

	// Update nozzle temperature
	nozzleCurrent, nozzleTarget := pc.controller.GetNozzleTemperature()

	pc.NozzleThermostat.CurrentTemperature.SetValue(nozzleCurrent)
	pc.NozzleThermostat.TargetTemperature.SetValue(nozzleTarget)

	// Update heating state: heating only if target > 0 AND current < target
	var nozzleState int
	if nozzleTarget > 0 && nozzleCurrent < nozzleTarget-1 {
		nozzleState = characteristic.CurrentHeatingCoolingStateHeat
	} else {
		nozzleState = characteristic.CurrentHeatingCoolingStateOff
	}
	pc.NozzleThermostat.CurrentHeatingCoolingState.SetValue(nozzleState)

	// Update target heating state
	var nozzleTargetState int
	if nozzleTarget > 0 {
		nozzleTargetState = characteristic.TargetHeatingCoolingStateHeat
	} else {
		nozzleTargetState = characteristic.TargetHeatingCoolingStateOff
	}
	pc.NozzleThermostat.TargetHeatingCoolingState.SetValue(nozzleTargetState)
}

// onPrintStatusUpdate is called when print status changes.
func (pc *PrinterControl) onPrintStatusUpdate() {
	_, _, active := pc.controller.GetPrintStatus()
	pc.PrintSwitch.On.SetValue(active)
}

// onFanUpdate is called when fan state is updated.
func (pc *PrinterControl) onFanUpdate() {
	fanOn := pc.controller.GetFanState()
	pc.Fan.On.SetValue(fanOn)
	log.Info.Printf("Fan state updated to: %v", fanOn)
}
