package api

import (
	"fmt"
	"net/http"
)

// PrinterStatusRequest is a request to get printer status.
type PrinterStatusRequest struct {
}

// PrinterStatusResponse is a response to a PrinterStatusRequest.
type PrinterStatusResponse struct {
	Data PrinterStatusResponseData `json:"data"`
}

// PrinterStatusResponseData is the response data of a PrinterStatusRequest.
type PrinterStatusResponseData struct {
	Connected         bool    `json:"connected"`
	BedTempCurrent    float64 `json:"bed_temp_current"`
	BedTempTarget     float64 `json:"bed_temp_target"`
	NozzleTempCurrent float64 `json:"nozzle_temp_current"`
	NozzleTempTarget  float64 `json:"nozzle_temp_target"`
	IsPrinting        bool    `json:"is_printing"`
	IsPaused          bool    `json:"is_paused"`
	Active            bool    `json:"active"` // Active = printing and not paused
	FanOn             bool    `json:"fan_on"`
}

// PrinterStatus returns the current printer status.
func (a *Api) PrinterStatus(w http.ResponseWriter, r *http.Request) {
	if a.PrinterController == nil {
		var resp = PrinterStatusResponse{
			Data: PrinterStatusResponseData{
				Connected: false,
			},
		}
		if err := WriteJSON(w, r, resp); err != nil {
			fmt.Println("responding failed", err)
		}
		return
	}

	bedCurrent, bedTarget := a.PrinterController.GetBedTemperature()
	nozzleCurrent, nozzleTarget := a.PrinterController.GetNozzleTemperature()
	isPrinting, isPaused, active := a.PrinterController.GetPrintStatus()
	fanOn := a.PrinterController.GetFanState()

	var resp = PrinterStatusResponse{
		Data: PrinterStatusResponseData{
			Connected:         true,
			BedTempCurrent:    bedCurrent,
			BedTempTarget:     bedTarget,
			NozzleTempCurrent: nozzleCurrent,
			NozzleTempTarget:  nozzleTarget,
			IsPrinting:        isPrinting,
			IsPaused:          isPaused,
			Active:            active,
			FanOn:             fanOn,
		},
	}

	if err := WriteJSON(w, r, resp); err != nil {
		fmt.Println("responding failed", err)
	}
}
