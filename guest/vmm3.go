// Copyright (c) 2020-present devguard GmbH

package main

import (
	"encoding/json"
	"time"
	"fmt"
	"github.com/kraudcloud/cradle/yeet"
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

	yc, err := yeet.New().WithLogger(log).Connect(urls...)

	if err != nil {
		exit(fmt.Errorf("cannot reach api: %s", err))
		return
	}

	go func() {
		var buf = make([]byte, 1024 * 1024 * 1024)
		for {
			n, err := yc.Read(buf)
			if err != nil {
				log.Errorf("vmm: %v", err)
				return
			}
			log.Infof("vmm: %v", string(buf[:n]))
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
