// Copyright (c) 2019-2021, AT&T Intellectual Property. All rights reseved.
//
// SPDX-License-Identifier: GPL-2.0-only
package ephemera

import (
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	eName := "net.vyatta.eng.vci.ephemeral.test"
	eModels := []string{
		"net.vyatta.eng.vci.ephemeral.test.v1",
		"net.vyatta.eng.vci.ephemeral.test.v2",
	}
	eStateGet := "/lib/vci-test-ephemeral/vci-test --action=get-state"
	eConfigGet := "/lib/vci-test-ephemeral/vci-test --action=get-config"
	eConfigSet := "/lib/vci-test-ephemeral/vci-test --action=commit"
	eConfigCheck := "/lib/vci-test-ephemeral/vci-test --action=validate"
	eRPCs := []string{
		"RPC/test/rpc1",
		"RPC/test/rpc2",
		"RPC/test/rpc3",
	}
	c, err := New(From("testdata/test.instance"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Name() != eName {
		t.Fatalf("Name should be %s not %s\n", eName, c.Name())
	}
	models := c.Models()
	if len(models) != len(eModels) {
		t.Fatalf("Didn't find the expected models\n")
	}
	for _, name := range eModels {
		model, ok := models[name]
		if !ok {
			t.Fatalf("Did not find the expected model %s\n", name)
		}
		intf, _ := model.Config()
		conf := intf.(*config)
		if conf.get != eConfigGet {
			t.Fatal("Did not have the correct Config/Get")
		}
		if conf.set != eConfigSet {
			t.Fatal("Did not have the correct Config/Set")
		}
		if conf.check != eConfigCheck {
			t.Fatal("Did not have the correct Config/Check")
		}
		intf, _ = model.State()
		state := intf.(*state)
		if state.get != eStateGet {
			t.Fatal("Did not have the correct State/Get")
		}
		rpcs, _ := model.RPC()
		for _, erpc := range eRPCs {
			parts := strings.Split(erpc, "/")
			module, name := parts[1], parts[2]
			mRPCs, ok := rpcs[module]
			if !ok {
				t.Fatal("Didn't find expected module", module)
			}
			_, ok = mRPCs[name]
			if !ok {
				t.Fatal("Didn't find expected rpc", name)
			}
		}
	}
}

func TestRunConfigGet(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrun.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	expected := `Component: net.vyatta.eng.vci.ephemeral.testrun
Model: net.vyatta.eng.vci.ephemeral.testrun.v1
Message: Config/Get
`

	out := string(conf.(*config).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}

func TestRunConfigSet(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrun.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Set(encodedString(""))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunConfigCheck(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrun.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Check(encodedString(""))
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunStateGet(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrun.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.State()
	if !ok {
		t.Fatal("no state")
	}

	expected := `Component: net.vyatta.eng.vci.ephemeral.testrun
Model: net.vyatta.eng.vci.ephemeral.testrun.v1
Message: State/Get
`

	out := string(conf.(*state).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}

func TestRunRPC(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrun.v1"]
	if !ok {
		t.Fatal("no model")
	}

	rpcs, ok := m.RPC()
	if !ok {
		t.Fatal("no rpc")
	}

	rpc := rpcs["test"]["rpc1"].(func(meta, in encodedString) (encodedString, error))
	expected := `Component: net.vyatta.eng.vci.ephemeral.testrun
Model: net.vyatta.eng.vci.ephemeral.testrun.v1
Message: RPC/test/rpc1
`

	out, err := rpc(encodedString("{}"), encodedString(""))
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n",
			string(out), expected)
	}
}

func TestRunStart(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Start()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunStop(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}

	err = c.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

func TestRunErrorConfigGet(t *testing.T) {
	c, err := New(From("testdata/testrunerr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunerr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	expected := ``

	out := string(conf.(*config).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}
func TestRunErrorConfigSet(t *testing.T) {
	c, err := New(From("testdata/testrunerr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunerr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Set(encodedString(""))
	if err == nil {
		t.Fatalf("expected error did not occur")
	}
}
func TestRunErrorConfigCheck(t *testing.T) {
	c, err := New(From("testdata/testrunerr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunerr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Check(encodedString(""))
	if err == nil {
		t.Fatalf("expected error did not occur")
	}
}
func TestRunErrorStateGet(t *testing.T) {
	c, err := New(From("testdata/testrunerr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunerr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	s, ok := m.State()
	if !ok {
		t.Fatal("no state")
	}

	expected := ``

	out := string(s.(*state).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}
func TestRunErrorRPC(t *testing.T) {
	c, err := New(From("testdata/testrunerr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunerr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	rpcs, ok := m.RPC()
	if !ok {
		t.Fatal("no rpc")
	}

	rpc := rpcs["test"]["rpc1"].(func(meta, in encodedString) (encodedString, error))

	_, err = rpc(encodedString("{}"), encodedString(""))
	if err == nil {
		t.Fatal("didn't get expected error")
	}
}

func TestRunStdErrorConfigGet(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunstderr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	expected := ``

	out := string(conf.(*config).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}
func TestRunStdErrorConfigSet(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunstderr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Set(encodedString(""))
	if err == nil {
		t.Fatalf("expected error did not occur")
	}
}
func TestRunStdErrorConfigCheck(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunstderr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	conf, ok := m.Config()
	if !ok {
		t.Fatal("no config")
	}

	err = conf.(*config).Check(encodedString(""))
	if err == nil {
		t.Fatalf("expected error did not occur")
	}
}
func TestRunStdErrorStateGet(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunstderr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	s, ok := m.State()
	if !ok {
		t.Fatal("no state")
	}

	expected := ``

	out := string(s.(*state).Get())
	if out != expected {
		t.Fatalf("got:\n%s\nexpected:\n%s\n", out, expected)
	}
}
func TestRunStdErrorRPC(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}

	m, ok := c.Models()["net.vyatta.eng.vci.ephemeral.testrunstderr.v1"]
	if !ok {
		t.Fatal("no model")
	}

	rpcs, ok := m.RPC()
	if !ok {
		t.Fatal("no rpc")
	}

	rpc := rpcs["test"]["rpc1"].(func(meta, in encodedString) (encodedString, error))

	_, err = rpc(encodedString("{}"), encodedString(""))
	if err == nil {
		t.Fatal("didn't get expected error")
	}
}

func TestRunStdErrorStart(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Start()
	if err == nil {
		t.Fatal("didn't get expected error")
	}
}

func TestRunStdErrorStop(t *testing.T) {
	c, err := New(From("testdata/testrunstderr.instance"))
	if err != nil {
		t.Fatal(err)
	}
	err = c.Stop()
	if err == nil {
		t.Fatal("didn't get expected error")
	}
}

func TestEqual(t *testing.T) {
	c, err := New(From("testdata/testrun.instance"))
	if err != nil {
		t.Fatal(err)
	}
	if !c.Equal(c) {
		t.Fatal("c != c")
	}
}
