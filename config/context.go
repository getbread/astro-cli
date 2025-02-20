package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/astronomer/astro-cli/pkg/printutil"
)

var (
	ErrInvalidConfigNoDomain = errors.New("cluster config invalid, no domain specified")
	ErrContextNotSet         = errors.New("no context set, have you authenticated to a cluster")
	ErrClusterNotSet         = errors.New("cluster not set, have you authenticated to this cluster")

	localhostDomain = "localhost"
	houstonDomain   = "houston"
	notApplicable   = "N/A"
)

// newTableOut construct new printutil.Table
func newTableOut() *printutil.Table {
	return &printutil.Table{
		Padding: []int{36, 36},
		Header:  []string{"CLUSTER", "WORKSPACE"},
	}
}

// Contexts holds all available Context structs in a map
type Contexts struct {
	Contexts map[string]Context `mapstructure:"contexts"`
}

// Context represents a single cluster context
type Context struct {
	Domain            string `mapstructure:"domain"`
	Workspace         string `mapstructure:"workspace"`
	LastUsedWorkspace string `mapstructure:"last_used_workspace"`
	Token             string `mapstructure:"token"`
}

// GetCurrentContext looks up current context and gets corresponding Context struct
func GetCurrentContext() (Context, error) {
	c := Context{}

	domain := CFG.Context.GetHomeString()
	if domain == "" {
		return Context{}, ErrContextNotSet
	}

	c.Domain = domain

	return c.GetContext()
}

// PrintContext prints current context to stdOut
func (c Context) PrintContext(out io.Writer) error {
	c, err := c.GetContext()
	if err != nil {
		return err
	}

	cluster := c.Domain
	if cluster == "" {
		cluster = notApplicable
	}

	workspace := c.Workspace
	if workspace == "" {
		workspace = notApplicable
	}
	tab := newTableOut()
	tab.AddRow([]string{cluster, workspace}, false)
	tab.Print(out)

	return nil
}

// PrintCurrentContext prints the current config context
func PrintCurrentContext(out io.Writer) error {
	c, err := GetCurrentContext()
	if err != nil {
		return err
	}

	err = c.PrintContext(out)
	if err != nil {
		return err
	}

	return nil
}

// GetContextKey allows a cluster domain to be used without interfering
// with viper's dot (.) notation for fetching configs by replacing with underscores (_)
func (c Context) GetContextKey() (string, error) {
	if c.Domain == "" {
		return "", ErrInvalidConfigNoDomain
	}

	return strings.Replace(c.Domain, ".", "_", -1), nil
}

// ContextExists checks if a cluster struct exists in config
// based on Cluster.Domain
// Returns a boolean indicating whether or not cluster exists
func (c Context) ContextExists() bool {
	key, err := c.GetContextKey()
	if err != nil {
		return false
	}

	return viperHome.IsSet("contexts" + "." + key)
}

// GetContext gets the full context from the specified Context receiver struct
// Returns based on Domain prop
func (c Context) GetContext() (Context, error) {
	key, err := c.GetContextKey()
	if err != nil {
		return c, err
	}

	if !c.ContextExists() {
		return c, ErrClusterNotSet
	}

	err = viperHome.UnmarshalKey("contexts"+"."+key, &c)
	if err != nil {
		return c, err
	}
	return c, nil
}

// GetContexts gets all contexts currently configured in the global config
// Returns a Contexts struct containing a map of all Context structs
func (c Contexts) GetContexts() (Contexts, error) {
	err := viperHome.Unmarshal(&c)
	if err != nil {
		return c, err
	}

	return c, nil
}

// SetContext saves Context to the config
func (c Context) SetContext() error {
	key, err := c.GetContextKey()
	if err != nil {
		return err
	}

	context := map[string]string{
		"token":               c.Token,
		"domain":              c.Domain,
		"workspace":           c.Workspace,
		"last_used_workspace": c.Workspace,
	}

	viperHome.Set("contexts"+"."+key, context)
	err = saveConfig(viperHome, HomeConfigFile)
	return err
}

// SetContextKey saves a single context key value pair
func (c Context) SetContextKey(key, value string) error {
	cKey, err := c.GetContextKey()
	if err != nil {
		return err
	}

	cfgPath := fmt.Sprintf("contexts.%s.%s", cKey, key)
	viperHome.Set(cfgPath, value)
	err = saveConfig(viperHome, HomeConfigFile)
	return err
}

// SwitchContext sets the current config context to the one matching the provided Context struct
func (c Context) SwitchContext() error {
	co, err := c.GetContext()
	if err != nil {
		return err
	}
	viperHome.Set("context", c.Domain)
	if err := saveConfig(viperHome, HomeConfigFile); err != nil {
		return err
	}
	tab := newTableOut()
	tab.AddRow([]string{co.Domain, co.Workspace}, false)
	tab.SuccessMsg = "\n Switched cluster"
	tab.Print(os.Stdout)

	return nil
}

// GetAPIURL returns full Houston API Url for the provided Context
func (c Context) GetAPIURL() string {
	if c.Domain == localhostDomain || c.Domain == houstonDomain {
		return CFG.LocalHouston.GetString()
	}

	return fmt.Sprintf(
		"%s://houston.%s:%s/v1",
		CFG.CloudAPIProtocol.GetString(),
		c.Domain,
		CFG.CloudAPIPort.GetString(),
	)
}

// GetWebsocketURL returns full Houston websocket Url for the provided Context
func (c Context) GetWebsocketURL() string {
	if c.Domain == localhostDomain || c.Domain == houstonDomain {
		return CFG.LocalHouston.GetString()
	}

	return fmt.Sprintf(
		"%s://houston.%s:%s/ws",
		CFG.CloudWSProtocol.GetString(),
		c.Domain,
		CFG.CloudAPIPort.GetString(),
	)
}

// GetAppURL returns full Houston API Url for the provided Context
func (c Context) GetAppURL() string {
	if c.Domain == localhostDomain || c.Domain == houstonDomain {
		return CFG.LocalOrbit.GetString()
	}

	return fmt.Sprintf(
		"%s://app.%s",
		CFG.CloudAPIProtocol.GetString(),
		c.Domain,
	)
}
