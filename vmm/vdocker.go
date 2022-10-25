// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
)

func (self *Vmm) HttpHandler() http.Handler {
	url, err := url.Parse(fmt.Sprintf("http://[%s]:1/", self.config.Network.FabricIp6))
	if err != nil {
		panic(err)
	}

	return httputil.NewSingleHostReverseProxy(url)
}
