// +build snp

package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/google/go-sev-guest/abi"
	"github.com/google/go-sev-guest/client"
	"os"
	"net/http"
	"bytes"
	"crypto/sha256"
)

func sev() {
	dev, err := client.OpenDevice()
	if err != nil {
		log.Warnf("sev: %v", err)
		return
	}
	defer dev.Close()


	var reportData [64]byte
	var hasher = sha256.New()
	json.NewEncoder(hasher).Encode(CONFIG)
	hasher.Sum(reportData[:0])

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


	//post it to the url given in env
	var url = ""
	for _, container := range CONFIG.Pod.Containers {
		for k,v := range container.Process.Env {
			if k == "KR_ATTESTATION_URL" {
				url = v
			}
		}
	}

	if url == "" {
		log.Info("sev: KR_ATTESTATION_URL not set")
		return
	}

	log.Info("sev: posting to KR_ATTESTATION_URL: ", url)

	jsonData, err := json.Marshal(map[string]interface{}{
		"Report": p,
		"Launch": CONFIG,
	})

	if err != nil {
		log.Error("sev: json.Marshal: ", err)
		return
	}

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		log.Error("sev: http.Post: ", err)
		return
	}

	if resp.StatusCode != http.StatusOK {
		exit(fmt.Errorf("%s returned : %v", url, resp.Status))
		return
	}

}
