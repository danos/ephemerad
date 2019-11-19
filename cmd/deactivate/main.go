// Copyright (c) 2019, AT&T Intellectual Property. All rights reseved.
//
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"flag"
	"log"

	rfc7951 "github.com/danos/encoding/rfc7951/data"
	"github.com/danos/vci"
)

var component string

func init() {
	flag.StringVar(
		&component,
		"component",
		"",
		"component name",
	)
}

func main() {
	flag.Parse()

	client, err := vci.Dial()
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	out := rfc7951.TreeNew()
	err = client.Call("ephemerad-v1", "deactivate",
		rfc7951.TreeNew().
			Assoc("/ephemerad-v1:component", component)).
		StoreOutputInto(out)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("stopped", component, out)
}
