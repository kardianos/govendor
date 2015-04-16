package kingpin

import "fmt"

type argGroup struct {
	args []*ArgClause
}

func newArgGroup() *argGroup {
	return &argGroup{}
}

func (a *argGroup) have() bool {
	return len(a.args) > 0
}

func (a *argGroup) Arg(name, help string) *ArgClause {
	arg := newArg(name, help)
	a.args = append(a.args, arg)
	return arg
}

func (a *argGroup) parse(context *ParseContext) error {
	i := 0
	var last *Token
	consumed := 0
	for i < len(a.args) {
		arg := a.args[i]
		token := context.Peek()
		if token.Type == TokenEOL {
			if consumed == 0 && arg.required {
				return fmt.Errorf("'%s' is required", arg.name)
			}
			break
		}

		var err error
		err = arg.parse(context)
		if err != nil {
			return err
		}

		if arg.consumesRemainder() {
			if last == context.Peek() {
				return fmt.Errorf("expected positional arguments <%s> but got '%s'", arg.name, last)
			}
			consumed++
		} else {
			i++
		}
		last = token
	}

	// Set defaults for all remaining args.
	for i < len(a.args) {
		arg := a.args[i]
		if arg.defaultValue != "" {
			if err := arg.value.Set(arg.defaultValue); err != nil {
				return fmt.Errorf("invalid default value '%s' for argument '%s'", arg.defaultValue, arg.name)
			}
		}
		i++
	}
	return nil
}

func (a *argGroup) init() error {
	required := 0
	seen := map[string]struct{}{}
	previousArgMustBeLast := false
	for i, arg := range a.args {
		if previousArgMustBeLast {
			return fmt.Errorf("Args() can't be followed by another argument '%s'", arg.name)
		}
		if arg.consumesRemainder() {
			previousArgMustBeLast = true
		}
		if _, ok := seen[arg.name]; ok {
			return fmt.Errorf("duplicate argument '%s'", arg.name)
		}
		seen[arg.name] = struct{}{}
		if arg.required && required != i {
			return fmt.Errorf("required arguments found after non-required")
		}
		if arg.required {
			required++
		}
		if err := arg.init(); err != nil {
			return err
		}
	}
	return nil
}

type ArgClause struct {
	parserMixin
	name         string
	help         string
	defaultValue string
	required     bool
	dispatch     Dispatch
}

func newArg(name, help string) *ArgClause {
	a := &ArgClause{
		name: name,
		help: help,
	}
	return a
}

func (a *ArgClause) consumesRemainder() bool {
	if r, ok := a.value.(remainderArg); ok {
		return r.IsCumulative()
	}
	return false
}

// Required arguments must be input by the user. They can not have a Default() value provided.
func (a *ArgClause) Required() *ArgClause {
	a.required = true
	return a
}

// Default value for this argument. It *must* be parseable by the value of the argument.
func (a *ArgClause) Default(value string) *ArgClause {
	a.defaultValue = value
	return a
}

func (a *ArgClause) Dispatch(dispatch Dispatch) *ArgClause {
	a.dispatch = dispatch
	return a
}

func (a *ArgClause) init() error {
	if a.required && a.defaultValue != "" {
		return fmt.Errorf("required argument '%s' with unusable default value", a.name)
	}
	if a.value == nil {
		return fmt.Errorf("no parser defined for arg '%s'", a.name)
	}
	return nil
}

func (a *ArgClause) parse(context *ParseContext) error {
	token := context.Peek()
	if token.Type == TokenArg {
		if err := a.value.Set(token.Value); err != nil {
			return err
		}
		if a.dispatch != nil {
			if err := a.dispatch(context); err != nil {
				return err
			}
		}
		context.Next()
	}
	return nil
}
