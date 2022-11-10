package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-sev-guest/abi"
	"github.com/google/go-sev-guest/client"
	"os"
)

func sev() {
	dev, err := client.OpenDevice()
	if err != nil {
		log.Warnf("sev: %v", err)
		return
	}
	defer dev.Close()

	var reportData [64]byte
	report, err := client.GetRawReportAtVmpl(dev, reportData, 0)
	if err != nil {
		log.Error("sev: GetRawReportAtVmpl: ", err)
		return
	}

	fmt.Printf(base64.StdEncoding.EncodeToString([]byte(report)))

	p, err := abi.ReportToProto(report)
	if err != nil {
		log.Error("sev: ReportToProto: ", err)
		return
	}

	json.NewEncoder(os.Stdout).Encode(p)
}
