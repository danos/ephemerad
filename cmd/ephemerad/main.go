// Copyright (c) 2019, AT&T Intellectual Property. All rights reseved.
//
// SPDX-License-Identifier: GPL-2.0-only
package main

import (
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"log/syslog"
	"os"
	"sync"

	rfc7951 "github.com/danos/encoding/rfc7951/data"
	"github.com/danos/ephemera"
	"github.com/danos/vci"
	"github.com/fsnotify/fsnotify"
	"jsouthworth.net/go/dyn"
	"jsouthworth.net/go/etm/agent"
	"jsouthworth.net/go/etm/atom"
	"jsouthworth.net/go/immutable/hashmap"
	"jsouthworth.net/go/immutable/vector"
)

var (
	elog        *log.Logger
	dlog        *log.Logger
	instanceDir string
)

func init() {
	elog, _ = syslog.NewLogger(syslog.LOG_ERR, 0)
	dlog, _ = syslog.NewLogger(syslog.LOG_DEBUG, 0)
	flag.StringVar(
		&instanceDir,
		"instance-dir",
		"/lib/vci/ephemera/instances",
		"directory with instance information",
	)
}

type component struct {
	meta    *ephemera.Component
	vci     vci.Component
	started *agent.Agent
}

func newComponent(meta *ephemera.Component, vci vci.Component) *component {
	return &component{
		meta:    meta,
		vci:     vci,
		started: agent.New(false),
	}
}

func (c *component) Run() error {
	ch := make(chan error)
	c.started.Send(func(isRunning bool) bool {
		var err error
		defer func() {
			ch <- err
			if !isRunning && err == nil {
				dlog.Println("Started listener for", c.meta.Name())
			}
		}()
		if isRunning {
			return isRunning
		}
		c.meta.Start()
		err = c.vci.Run()
		if err == nil {
			return true
		}
		return false
	})
	return <-ch
}

func (c *component) Running() bool {
	return c.started.Deref().(bool)
}

func (c *component) Stop() error {
	ch := make(chan error)
	c.started.Send(func(isRunning bool) bool {
		var err error
		defer func() {
			ch <- err
			if isRunning && err == nil {
				dlog.Println("Stopped listener for", c.meta.Name())
			}
		}()
		if !isRunning {
			return isRunning
		}
		c.meta.Stop()
		err = c.vci.Stop()
		if err == nil {
			return false
		}
		return true
	})
	return <-ch
}

func readAllComponents(instanceDir string) *hashmap.Map {
	return hashmap.Empty().
		Transform(func(cs *hashmap.TMap) *hashmap.TMap {
			dir, err := ioutil.ReadDir(instanceDir)
			if err != nil {
				return cs
			}
			for _, fi := range dir {
				if fi.IsDir() {
					continue
				}
				name := instanceDir + "/" + fi.Name()
				comp, err := ephemera.New(
					ephemera.From(name),
				)
				if err != nil {
					elog.Printf("%s: %s", name, err)
					continue
				}
				cs = cs.Assoc(comp.Name(), newComponent(
					comp,
					createVCIComponent(comp),
				))
			}
			return cs
		})
}

func createVCIComponent(comp *ephemera.Component) vci.Component {
	c := vci.NewComponent(comp.Name())
	for name, model := range comp.Models() {
		m := c.Model(name)
		conf, ok := model.Config()
		if ok {
			m.Config(conf)
		}
		state, ok := model.State()
		if ok {
			m.State(state)
		}
		modules, ok := model.RPC()
		if !ok {
			continue
		}
		for module, rpcs := range modules {
			m.RPC(module, rpcs)
		}
	}
	return c
}

func syncComponents(
	key string,
	a *atom.Atom,
	old, new *hashmap.Map,
) {
	type action struct {
		op     func() error
		name   string
		opname string
	}
	actions := vector.Empty().AsTransient()
	old.Range(func(name string, comp *component) {
		if new.Contains(name) {
			return
		}
		actions = actions.Append(&action{
			op:     comp.Stop,
			name:   name,
			opname: "stopp",
		})
	})
	new.Range(func(name string, comp *component) {
		val, ok := old.Find(name)
		if !ok || dyn.Equal(comp.meta, val.(*component).meta) {
			return
		}
		// If the meta components differ then we need to stop the
		// old one. The new one will be started with activation
		// on the next call. We can't start the new one now because
		// if the component file were added during package installation
		// the bus may not be setup correctly yet.
		actions = actions.Append(&action{
			op:     val.(*component).Stop,
			name:   name,
			opname: "stopp",
		})
	})
	actions.Range(func(_ int, act *action) {
		dlog.Printf("Instance sync: %sing %s\n", act.opname, act.name)
		err := act.op()
		if err == nil {
			return
		}
		elog.Printf("Error %sing component on sync: %s: %s\n",
			act.opname, act.name, err)
	})
}

func watchInstanceDirectory(
	instanceDir string,
	managedComponents *atom.Atom,
) {
	swapper := func(old *hashmap.Map) *hashmap.Map {
		new := readAllComponents(instanceDir)
		new = new.Transform(func(t *hashmap.TMap) *hashmap.TMap {
			t.Range(func(name string, comp *component) {
				// If the meta components are the
				// same, preserve the original vci
				// component.
				oldComp, ok := old.Find(name)
				if !ok {
					return
				}
				if dyn.Equal(comp.meta,
					oldComp.(*component).meta) {
					t.Assoc(name, oldComp)
				}
			})
			return t
		})
		return new
	}

	handleEvent := func(event fsnotify.Event) {
		switch {
		case event.Op&fsnotify.Chmod == fsnotify.Chmod:
		default:
			managedComponents.Swap(swapper)
		}
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	watcher.Add(instanceDir)

	var ready sync.WaitGroup
	ready.Add(1)
	go func() {
		ready.Done()
		for {
			select {
			case event := <-watcher.Events:
				handleEvent(event)
			case err := <-watcher.Errors:
				elog.Println("watch instances:", err)
			}
		}
	}()
	ready.Wait()
}

type rpc struct {
	managedComponents *atom.Atom
}

func (r *rpc) Activate(in *rfc7951.Tree) (*rfc7951.Tree, error) {
	name := in.At("/ephemerad-v1:component").ToString()

	cs := r.managedComponents.Deref().(*hashmap.Map)
	comp, found := cs.Find(name)
	if !found {
		return nil, errors.New("no component by the name " +
			name + " found")
	}
	err := comp.(*component).Run()
	if err != nil {
		return nil, err
	}

	return rfc7951.TreeNew(), nil
}

func (r *rpc) Deactivate(in *rfc7951.Tree) (*rfc7951.Tree, error) {
	name := in.At("/ephemerad-v1:component").ToString()

	cs := r.managedComponents.Deref().(*hashmap.Map)
	comp, found := cs.Find(name)
	if !found {
		return nil, errors.New("no component by the name " +
			name + " found")
	}

	err := comp.(*component).Stop()
	if err != nil {
		return nil, err
	}

	return rfc7951.TreeNew(), nil
}

func main() {
	flag.Parse()
	// Ensure that the instanceDir exists
	err := os.MkdirAll(instanceDir, 0644)
	if err != nil {
		elog.Fatal(err)
	}

	// Load initial components
	components := readAllComponents(instanceDir)
	// Store them in an atomic variable
	managedComponents := atom.New(components)
	// Register a handler to sync them to the system when they change
	managedComponents.Watch("sync-components", syncComponents)
	// register file system watcher for component updates
	watchInstanceDirectory(instanceDir, managedComponents)

	// Component and datamodel for ephemerad.
	ephemerad := vci.NewComponent("net.vyatta.vci.ephemera")
	ephemerad.Model("net.vyatta.vci.ephemera.v1").RPC("ephemerad-v1", &rpc{
		managedComponents: managedComponents,
	})
	err = ephemerad.Run()
	if err != nil {
		elog.Fatal(err)
	}

	// Wait (forever)
	ephemerad.Wait()
}
