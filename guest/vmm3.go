// Copyright (c) 2020-present devguard GmbH

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/kraudcloud/cradle/yeet"
	"net/http"
	"time"
)

func vmm3() {

	if CONFIG.Role == nil {
		log.Println("cradle: no vmm role, not connecting to api")
		return
	}

	urls := []string{}
	for _, url := range CONFIG.Role.Api {
		urls = append(urls, fmt.Sprintf("%s/apis/kr.vmm/v1/pod/%s/cradle.yeet.json", url, CONFIG.ID))
	}

	yc, err := yeet.New().
		WithHeader("Authorization", fmt.Sprintf("Bearer %s", CONFIG.Role.Token)).
		WithLogger(log).
		Connect(urls...)

	if err != nil {
		exit(fmt.Errorf("cannot reach api: %s", err))
		return
	}

	go func() {
		var buf = make([]byte, 1*1024*1024)
		for {
			n, err := yc.Read(buf)
			if err != nil {
				log.Errorf("vmm: %v", err)
				return
			}

			var st Status
			err = json.Unmarshal(buf[:n], &st)
			if err != nil {
				log.Errorf("vmm: %v", err)
				continue
			}

			if st.V != nil {
				UpdateDNS(st.V)
				UpdateServices(st.V)
				continue
			}
		}
	}()

	for {
		time.Sleep(time.Second)

		report := map[string]interface{}{
			"Pod": map[string]interface{}{
				"IP6": CONFIG.Network.FabricIp6,
			},
		}

		js, _ := json.Marshal(report)
		yc.Write(js)

	}
}

func reportContainerState(
	id string,
	state uint32,
	code int,
	msg string,
	lastlog []byte,
) {

	if CONFIG.Role == nil {
		log.Println("cradle: no vmm role, not connecting to api")
		return
	}

	urls := []string{}
	for _, url := range CONFIG.Role.Api {
		urls = append(urls, fmt.Sprintf("%s/apis/kr.vmm/v1/container/%s/report.json", url, id))
	}

	mm := map[string]interface{}{
		"State":    state,
		"Message":  msg,
	}

	if code != -1 {
		mm["ExitCode"] = code
	}

	if len(lastlog) > 0 {
		if len(lastlog) > 2000 {
			lastlog = lastlog[len(lastlog)-2000:]
		}
		mm["Log"] = base64.StdEncoding.EncodeToString(lastlog)
	}

	j, _ := json.Marshal(mm)

	for i := 0; i < 3; i++ {
		for _, url := range urls {

			req, err := http.NewRequest("POST", url, bytes.NewReader(j))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", CONFIG.Role.Token))
			req.Header.Set("Content-Type", "application/json")

			rsp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Debugf("vmm: failure reporting state to %s: %s", url, err)
				continue
			}
			rsp.Body.Close()

			if rsp.StatusCode < 400 {
				return
			}

			log.Printf("vmm: failure reporting state to %s: %d", url, rsp.StatusCode)

			time.Sleep(300 * time.Millisecond)
		}
	}

}

func reportExit(reason string) {

	if CONFIG.Role == nil {
		log.Println("cradle: no vmm role, not connecting to api")
		return
	}

	urls := []string{}
	for _, url := range CONFIG.Role.Api {
		urls = append(urls, fmt.Sprintf("%s/apis/kr.vmm/v1/pod/%s/report.json", url, CONFIG.ID))
	}

	mm := map[string]interface{}{
		"Reason": reason,
	}

	j, _ := json.Marshal(mm)

	for i := 0; i < 10; i++ {
		for _, url := range urls {

			req, err := http.NewRequest("POST", url, bytes.NewReader(j))
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", CONFIG.Role.Token))
			req.Header.Set("Content-Type", "application/json")

			rsp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Debugf("vmm: failure reporting state to %s: %s", url, err)
				continue
			}
			rsp.Body.Close()

			if rsp.StatusCode == 201 || rsp.StatusCode == 200 {
				return
			}

			log.Printf("vmm: failure reporting state to %s: %d", url, rsp.StatusCode)

			//FIXME when we exit too early, network is not yet ready
			time.Sleep(time.Second)
		}
	}

}
