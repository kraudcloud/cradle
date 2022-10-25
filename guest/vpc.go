// Copyright (c) 2020-present devguard GmbH

package main

import ()

type ServicePort struct {
	Protocol   string   `json:"p"`
	ListenPort uint16   `json:"l"`
	To6        []string `json:"6,omitempty"`
}

type Service struct {
	Names     []string      `json:"n"`
	Namespace string        `json:"d"`
	IP4       string        `json:"4"`
	IP6       string        `json:"6"`
	Ports     []ServicePort `json:"p,omitempty"`
}

type Pod struct {
	Names     []string `json:"n"`
	Namespace string   `json:"d"`
	IP6       string   `json:"6"`
}

type Vpc struct {
	Pods     map[string]*Pod     `json:"pod,omitempty"`
	Services map[string]*Service `json:"srv,omitempty"`
}

type Status struct {
	V *Vpc `json:"V,omitempty"`
}
