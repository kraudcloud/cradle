package main

import (
	"github.com/google/go-sev-guest/client"
	"github.com/google/go-sev-guest/abi"
	"encoding/base64"
	"fmt"
	"encoding/json"
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
	report, certs, err := client.GetRawExtendedReportAtVmpl(dev, reportData, 0)
	if err != nil {
		log.Error("sev: GetRawExtendedReportAtVmpl: ", err)
		return
	}

	_ = certs

	fmt.Printf(base64.StdEncoding.EncodeToString([]byte(report)))
	//reportJ.Certs = base64.StdEncoding.EncodeToString([]byte(certs))


	p, err := abi.ReportToProto(report)
	if err != nil {
		log.Error("sev: ReportToProto: ", err)
		return
	}

	json.NewEncoder(os.Stdout).Encode(p)
}
