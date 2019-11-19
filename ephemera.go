// Copyright (c) 2019, AT&T Intellectual Property. All rights reseved.
//
// SPDX-License-Identifier: GPL-2.0-only
package ephemera

import (
	"bytes"
	"encoding/json"
	"log"
	"log/syslog"
	"os"
	"os/exec"
	"strings"

	"github.com/danos/mgmterror"
	"github.com/go-ini/ini"
	"jsouthworth.net/go/dyn"
)

var (
	elog *log.Logger
	dlog *log.Logger
)

func init() {
	var err error
	elog, err = syslog.NewLogger(syslog.LOG_ERR, 0)
	if err != nil {
		elog = log.New(os.Stderr, "", 0)
	}
	dlog, err = syslog.NewLogger(syslog.LOG_DEBUG, 0)
	if err != nil {
		dlog = log.New(os.Stdout, "", 0)
	}
}

type encodedString []byte

func (s *encodedString) UnmarshalJSON(data []byte) error {
	*s = encodedString(data)
	return nil
}

func (s encodedString) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return s, nil
}

func (s *encodedString) UnmarshalRFC7951(data []byte) error {
	*s = encodedString(data)
	return nil
}

func (s encodedString) MarshalRFC7951() ([]byte, error) {
	if s == nil {
		return []byte("null"), nil
	}
	return s, nil
}

type config struct {
	compName  string
	modelName string
	get       string
	set       string
	check     string
}

func configNew(compName, modelName string, section *ini.Section) *config {
	getKey := section.Key("Config/Get")
	setKey := section.Key("Config/Set")
	chkKey := section.Key("Config/Check")
	if getKey == nil && setKey == nil && chkKey == nil {
		return nil
	}
	return &config{
		compName:  compName,
		modelName: modelName,
		get:       getKey.MustString(""),
		set:       setKey.MustString(""),
		check:     chkKey.MustString(""),
	}
}

func (c *config) Get() encodedString {
	if c.get == "" {
		//TODO: read/write cache from/to disk
		return []byte{}
	}
	getArgs := strings.Split(c.get, " ")
	stdErr := bytes.NewBuffer(nil)
	cmd := exec.Command(getArgs[0], getArgs[1:]...)
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.compName, c.modelName, "Config/Get")

	buf, err := cmd.Output()
	if err != nil {
		merr := unpackError(stdErr)
		elog.Println("Error for", cmd.Env, merr)
		return []byte{}
	}
	return buf
}

func (c *config) Set(in encodedString) error {
	if c.set == "" {
		return nil
	}
	stdIn := bytes.NewBuffer([]byte(in))
	stdErr := bytes.NewBuffer(nil)

	setArgs := strings.Split(c.set, " ")
	cmd := exec.Command(setArgs[0], setArgs[1:]...)
	cmd.Stdin = stdIn
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.compName, c.modelName, "Config/Set")

	out, err := cmd.Output()
	if len(out) != 0 {
		dlog.Printf("Output for %s\n%s\n", cmd.Env, string(out))
	}
	if err != nil {
		merr := unpackError(stdErr)
		elog.Println("Error for", cmd.Env, merr)
		return merr
	}
	return nil
}

func (c *config) Check(in encodedString) error {
	if c.check == "" {
		return nil
	}
	stdIn := bytes.NewBuffer([]byte(in))
	stdErr := bytes.NewBuffer(nil)

	checkArgs := strings.Split(c.check, " ")
	cmd := exec.Command(checkArgs[0], checkArgs[1:]...)
	cmd.Stdin = stdIn
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.compName, c.modelName, "Config/Check")

	out, err := cmd.Output()
	if len(out) != 0 {
		dlog.Printf("Output for %s\n%s\n", cmd.Env, string(out))
	}
	if err != nil {
		merr := unpackError(stdErr)
		elog.Println("Error for", cmd.Env, merr)
		return merr
	}
	return nil
}

func (c *config) Equal(other interface{}) bool {
	oc, isConfig := other.(*config)
	return isConfig &&
		c.get == oc.get &&
		c.set == oc.set &&
		c.check == oc.check
}

type state struct {
	compName  string
	modelName string
	get       string
}

func stateNew(compName, modelName string, section *ini.Section) *state {
	getKey := section.Key("State/Get")
	if getKey == nil {
		return nil
	}
	return &state{
		compName:  compName,
		modelName: modelName,
		get:       getKey.MustString(""),
	}
}

func (c *state) Get() encodedString {
	if c.get == "" {
		return []byte{}
	}
	getArgs := strings.Split(c.get, " ")
	stdErr := bytes.NewBuffer(nil)
	cmd := exec.Command(getArgs[0], getArgs[1:]...)
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.compName, c.modelName, "State/Get")

	buf, err := cmd.Output()
	if err != nil {
		merr := unpackError(stdErr)
		elog.Println("Error for", cmd.Env, merr)
		return []byte{}
	}
	return buf
}

func (c *state) Equal(other interface{}) bool {
	os, isState := other.(*state)
	return isState &&
		c.get == os.get
}

type rpc struct {
	compName  string
	modelName string
	modules   map[string]map[string]string
}

func rpcNew(compName, modelName string, section *ini.Section) *rpc {
	modules := make(map[string]map[string]string)
	for _, key := range section.Keys() {
		if !strings.HasPrefix(key.Name(), "RPC/") {
			continue
		}
		parts := strings.Split(key.Name(), "/")
		if len(parts) != 3 {
			dlog.Println("skipping", parts)
			continue
		}
		module, name := parts[1], parts[2]
		rpcs, ok := modules[module]
		if !ok {
			rpcs = make(map[string]string)
		}
		rpcs[name] = key.String()
		modules[module] = rpcs
	}
	if len(modules) == 0 {
		return nil
	}
	return &rpc{
		compName:  compName,
		modelName: modelName,
		modules:   modules,
	}
}

func (r *rpc) genRpc(module, name, rpc string) interface{} {
	return func(in encodedString) (encodedString, error) {
		stdIn := bytes.NewBuffer([]byte(in))
		stdErr := bytes.NewBuffer(nil)

		args := strings.Split(rpc, " ")
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stdin = stdIn
		cmd.Stderr = stdErr
		cmd.Env = genEnvironment(r.compName, r.modelName,
			strings.Join([]string{"RPC", module, name}, "/"))

		out, err := cmd.Output()
		if err != nil {
			merr := unpackError(stdErr)
			elog.Println("Error for", cmd.Env, merr)
			return []byte{}, merr
		}
		return out, nil
	}
}
func (r *rpc) genRpcs() map[string]map[string]interface{} {
	if r == nil {
		return nil
	}
	out := make(map[string]map[string]interface{})
	for module, rpcs := range r.modules {
		funcs := make(map[string]interface{})
		for name, rpc := range rpcs {
			funcs[name] = r.genRpc(module, name, rpc)
		}
		out[module] = funcs
	}
	return out
}

func (r *rpc) Equal(other interface{}) bool {
	or, isRPC := other.(*rpc)
	if !isRPC || len(or.modules) != len(r.modules) {
		return false
	}
	for mod, names := range r.modules {
		oNames, ok := or.modules[mod]
		if !ok {
			return false
		}
		for name, script := range names {
			if oNames[name] != script {
				return false
			}
		}
	}
	for mod, names := range or.modules {
		rNames, ok := r.modules[mod]
		if !ok {
			return false
		}
		for name, script := range names {
			if rNames[name] != script {
				return false
			}
		}
	}
	return true
}

type Model struct {
	name string

	config *config
	state  *state
	rpc    *rpc
}

func (c *Model) Config() (interface{}, bool) {
	return c.config, c.config != nil
}

func (c *Model) State() (interface{}, bool) {
	return c.state, c.state != nil
}

func (c *Model) RPC() (map[string]map[string]interface{}, bool) {
	return c.rpc.genRpcs(), c.rpc != nil
}

func (c *Model) Equal(other interface{}) bool {
	om, isModel := other.(*Model)
	return isModel &&
		c.name == om.name &&
		dyn.Equal(c.config, om.config) &&
		dyn.Equal(c.state, om.state) &&
		dyn.Equal(c.rpc, om.rpc)
}

func modelNew(compName, name string, section *ini.Section) *Model {
	m := &Model{name: name}
	m.config = configNew(compName, name, section)
	m.state = stateNew(compName, name, section)
	m.rpc = rpcNew(compName, name, section)
	return m
}

type Component struct {
	instanceFile string
	name         string

	start  string
	stop   string
	models map[string]*Model
}

func (c *Component) instantiate() error {
	cfg, err := ini.Load(c.instanceFile)
	if err != nil {
		return err
	}
	c.name = cfg.Section("Component").Key("Name").MustString("")
	c.start = cfg.Section("Component").Key("Start").MustString("")
	c.stop = cfg.Section("Component").Key("Stop").MustString("")
	for _, section := range cfg.Sections() {
		if !strings.HasPrefix(section.Name(), "Model ") {
			continue
		}
		modelName := strings.Split(section.Name(), " ")[1]
		c.models[modelName] = modelNew(c.name, modelName, section)
	}
	return nil
}

func (c *Component) Name() string {
	return c.name
}

func (c *Component) Models() map[string]*Model {
	return c.models
}

func (c *Component) Equal(other interface{}) bool {
	oc, isComponent := other.(*Component)
	return isComponent &&
		c.name == oc.name &&
		c.start == oc.start &&
		c.stop == oc.stop &&
		c.equalModels(oc)
}

func (c *Component) Start() error {
	if c.start == "" {
		return nil
	}
	startArgs := strings.Split(c.start, " ")
	stdErr := bytes.NewBuffer(nil)
	cmd := exec.Command(startArgs[0], startArgs[1:]...)
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.name, "", "Start")

	buf, err := cmd.Output()
	if len(buf) != 0 {
		dlog.Printf("Output for %s\n%s\n", cmd.Env, string(buf))
	}

	if err == nil {
		return nil
	}

	merr := unpackError(stdErr)
	elog.Println("Error for", cmd.Env, merr)
	return merr
}

func (c *Component) Stop() error {
	if c.stop == "" {
		return nil
	}
	stopArgs := strings.Split(c.stop, " ")
	stdErr := bytes.NewBuffer(nil)
	cmd := exec.Command(stopArgs[0], stopArgs[1:]...)
	cmd.Stderr = stdErr
	cmd.Env = genEnvironment(c.name, "", "Stop")

	buf, err := cmd.Output()
	if len(buf) != 0 {
		dlog.Printf("Output for %s\n%s\n", cmd.Env, string(buf))
	}

	if err == nil {
		return nil
	}

	merr := unpackError(stdErr)
	elog.Println("Error for", cmd.Env, merr)
	return merr
}

func (c *Component) equalModels(other *Component) bool {
	if len(c.models) != len(other.models) {
		return false
	}
	for k, v := range c.models {
		if !dyn.Equal(v, other.models[k]) {
			return false
		}
	}
	for k, v := range other.models {
		if !dyn.Equal(v, c.models[k]) {
			return false
		}
	}
	return true
}

func genEnvironment(compName, modelName, operation string) []string {
	return []string{
		"VCI_COMPONENT_NAME=" + compName,
		"VCI_MODEL_NAME=" + modelName,
		"EPHEMERA_MESSAGE=" + operation,
	}
}

func unpackError(stdErr *bytes.Buffer) error {
	var merr mgmterror.MgmtError
	err := json.Unmarshal(stdErr.Bytes(), &merr)
	if err != nil {
		err = mgmterror.NewExecError(nil, stdErr.String())
	} else {
		err = &merr
	}
	return err
}

type Opt func(*Component)

func From(file string) Opt {
	return func(c *Component) {
		c.instanceFile = file
	}
}

func New(opts ...Opt) (*Component, error) {
	c := &Component{
		models: make(map[string]*Model),
	}
	for _, opt := range opts {
		opt(c)
	}
	err := c.instantiate()
	if err != nil {
		return nil, err
	}
	return c, nil
}
