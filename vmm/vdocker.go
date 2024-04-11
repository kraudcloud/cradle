// Copyright (c) 2020-present devguard GmbH

package vmm

import (
	"net/http"
)

func (self *Vmm) HttpHandler() http.Handler {

	// FIXME how to contact cradle?

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotImplemented)
	})

	//url, err := url.Parse(fmt.Sprintf("http://[%s]:1/", fabricIp6))
	//if err != nil {
	//	panic(err)
	//}

	//return httputil.NewSingleHostReverseProxy(url)
}
