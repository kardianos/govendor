package kingpin

import (
	"fmt"
	"os"
	"strings"
)

type cmdGroup struct {
	app          *Application
	parent       *CmdClause
	commands     map[string]*CmdClause
	commandOrder []*CmdClause
}

func newCmdGroup(app *Application) *cmdGroup {
	return &cmdGroup{
		app:      app,
		commands: make(map[string]*CmdClause),
	}
}

func (c *cmdGroup) flattenedCommands() (out []*CmdClause) {
	for _, cmd := range c.commandOrder {
		if len(cmd.commands) == 0 {
			out = append(out, cmd)
		}
		out = append(out, cmd.flattenedCommands()...)
	}
	return
}

func (c *cmdGroup) addCommand(name, help string) *CmdClause {
	cmd := newCommand(c.app, name, help)
	c.commands[name] = cmd
	c.commandOrder = append(c.commandOrder, cmd)
	return cmd
}

func (c *cmdGroup) init() error {
	seen := map[string]bool{}
	for _, cmd := range c.commandOrder {
		if seen[cmd.name] {
			return fmt.Errorf("duplicate command '%s'", cmd.name)
		}
		seen[cmd.name] = true
		if err := cmd.init(); err != nil {
			return err
		}
	}
	return nil
}

func (c *cmdGroup) parse(context *ParseContext) (selected []string, _ error) {
	token := context.Peek()
	if token.Type == TokenEOL {
		return nil, nil
	}
	if token.Type != TokenArg {
		return nil, fmt.Errorf("expected command but got '%s'", token)
	}
	cmd, ok := c.commands[token.String()]
	if !ok {
		return nil, fmt.Errorf("no such command '%s'", token)
	}
	context.Next()
	context.SelectedCommand = cmd.name
	selected, err := cmd.parse(context)
	if err == nil {
		selected = append([]string{token.String()}, selected...)
	}
	return selected, err
}

func (c *cmdGroup) have() bool {
	return len(c.commands) > 0
}

type CmdClauseValidator func(*CmdClause) error

// A CmdClause is a single top-level command. It encapsulates a set of flags
// and either subcommands or positional arguments.
type CmdClause struct {
	*flagGroup
	*argGroup
	*cmdGroup
	app       *Application
	name      string
	help      string
	dispatch  Dispatch
	validator CmdClauseValidator
}

func newCommand(app *Application, name, help string) *CmdClause {
	c := &CmdClause{
		flagGroup: newFlagGroup(),
		argGroup:  newArgGroup(),
		cmdGroup:  newCmdGroup(app),
		app:       app,
		name:      name,
		help:      help,
	}
	c.Flag("help", "Show help on this command.").Hidden().Dispatch(c.onHelp).Bool()
	return c
}

// Validate sets a validation function to run when parsing.
func (c *CmdClause) Validate(validator CmdClauseValidator) *CmdClause {
	c.validator = validator
	return c
}

func (c *CmdClause) FullCommand() string {
	out := []string{c.name}
	for p := c.parent; p != nil; p = p.parent {
		out = append([]string{p.name}, out...)
	}
	return strings.Join(out, " ")
}

func (c *CmdClause) onHelp(context *ParseContext) error {
	c.app.CommandUsage(os.Stderr, c.FullCommand())
	os.Exit(0)
	return nil
}

// Command adds a new sub-command.
func (c *CmdClause) Command(name, help string) *CmdClause {
	cmd := c.addCommand(name, help)
	cmd.parent = c
	return cmd
}

func (c *CmdClause) Dispatch(dispatch Dispatch) *CmdClause {
	c.dispatch = dispatch
	return c
}

func (c *CmdClause) init() error {
	if err := c.flagGroup.init(); err != nil {
		return err
	}
	if c.argGroup.have() && c.cmdGroup.have() {
		return fmt.Errorf("can't mix Arg()s with Command()s")
	}
	if err := c.argGroup.init(); err != nil {
		return err
	}
	if err := c.cmdGroup.init(); err != nil {
		return err
	}
	return nil
}

func (c *CmdClause) parse(context *ParseContext) (selected []string, _ error) {
	err := c.flagGroup.parse(context, false)
	if err != nil {
		return nil, err
	}
	if context.SelectedCommand != "help" {
		if c.cmdGroup.have() {
			selected, err = c.cmdGroup.parse(context)
		} else if c.argGroup.have() {
			err = c.argGroup.parse(context)
		}
	}
	if err == nil && c.dispatch != nil {
		err = c.dispatch(context)
	}
	if c.validator != nil {
		err = c.validator(c)
	}
	return selected, err
}
