package targets

import (
	"fmt"
	"net/http"
	"os"

	"github.com/hashicorp/boundary/api"
	"github.com/hashicorp/boundary/api/targets"
	"github.com/hashicorp/boundary/internal/cmd/base"
	"github.com/hashicorp/boundary/internal/cmd/common"
	"github.com/hashicorp/boundary/internal/types/resource"
	"github.com/hashicorp/boundary/sdk/strutil"
	"github.com/kr/pretty"
	"github.com/mitchellh/cli"
	"github.com/posener/complete"
)

var _ cli.Command = (*Command)(nil)
var _ cli.CommandAutocomplete = (*Command)(nil)

type Command struct {
	*base.Command

	Func string

	flagHostSets []string
	flagHostId   string
}

func (c *Command) Synopsis() string {
	switch c.Func {
	case "create":
		return "Create target resources within Boundary"
	case "update":
		return "Update target resources within Boundary"
	default:
		return common.SynopsisFunc(c.Func, "target")
	}
}

var flagsMap = map[string][]string{
	"authorize":        {"id", "host-id"},
	"read":             {"id"},
	"delete":           {"id"},
	"list":             {"scope-id"},
	"add-host-sets":    {"id", "host-set", "version"},
	"remove-host-sets": {"id", "host-set", "version"},
	"set-host-sets":    {"id", "host-set", "version"},
}

func (c *Command) Help() string {
	helpMap := common.HelpMap("target")
	var helpStr string
	switch c.Func {
	case "":
		return base.WrapForHelpText([]string{
			"Usage: boundary targets [sub command] [options] [args]",
			"",
			"  This command allows operations on Boundary target resources. Example:",
			"",
			"    Read a target:",
			"",
			`      $ boundary targets read -id ttcp_1234567890`,
			"",
			"  Please see the targets subcommand help for detailed usage information.",
		})
	case "create":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary targets create [type] [sub command] [options] [args]",
			"",
			"  This command allows create operations on Boundary target resources. Example:",
			"",
			"    Create a tcp-type target:",
			"",
			`      $ boundary targets create tcp -name prodops -description "For ProdOps usage"`,
			"",
			"  Please see the typed subcommand help for detailed usage information.",
		})
	case "update":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary targets update [type] [sub command] [options] [args]",
			"",
			"  This command allows update operations on Boundary target resources. Example:",
			"",
			"    Update a tcp-type target:",
			"",
			`      $ boundary targets update tcp -id ttcp_1234567890 -name devops -description "For DevOps usage"`,
			"",
			"  Please see the typed subcommand help for detailed usage information.",
		})
	case "add-host-sets":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary target add-host-sets [options] [args]",
			"",
			"  This command allows adding host-set resources to target resources. Example:",
			"",
			"    Add host-set resources to a tcp-type target:",
			"",
			`      $ boundary targets add-host-sets -id ttcp_1234567890 -host-set hsst_1234567890 -host-set hsst_0987654321`,
		})
	case "remove-host-sets":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary target remove-host-sets [options] [args]",
			"",
			"  This command allows removing host-set resources from target resources. Example:",
			"",
			"    Remove host-set resources from a tcp-type target:",
			"",
			`      $ boundary targets add-host-sets -id ttcp_1234567890 -host hsst_1234567890 -host-set hsst_0987654321`,
		})
	case "set-host-sets":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary target set-host-sets [options] [args]",
			"",
			"  This command allows setting the complete set of host-set resources on a target resource. Example:",
			"",
			"    Set host-set resources on a tcp-type target:",
			"",
			`      $ boundary targets set-host-sets -id ttcp_1234567890 -host-set hsst_1234567890`,
		})
	case "authorize":
		helpStr = base.WrapForHelpText([]string{
			"Usage: boundary target authorize [options] [args]",
			"",
			"  This command allows fetching session authorization credentials against a target. Example:",
			"",
			"    Set host-set resources on a tcp-type target:",
			"",
			`      $ boundary targets authorize -id ttcp_1234567890`,
		})
	default:
		helpStr = helpMap[c.Func]()
	}
	return helpStr + c.Flags().Help()
}

func (c *Command) Flags() *base.FlagSets {
	set := c.FlagSet(base.FlagSetHTTP | base.FlagSetClient | base.FlagSetOutputFormat)

	f := set.NewFlagSet("Command Options")
	common.PopulateCommonFlags(c.Command, f, resource.Target.String(), flagsMap[c.Func])

	for _, name := range flagsMap[c.Func] {
		switch name {
		case "host-set":
			f.StringSliceVar(&base.StringSliceVar{
				Name:   "host-set",
				Target: &c.flagHostSets,
				Usage:  "The host-set resources to add, remove, or set. May be specified multiple times.",
			})
		case "host-id":
			f.StringVar(&base.StringVar{
				Name:   "host-id",
				Target: &c.flagHostId,
				Usage:  "The ID of a specific host to connect to out of the hosts from the target's host sets. If not specified, one is chosen at random.",
			})
		}
	}

	return set
}

func (c *Command) AutocompleteArgs() complete.Predictor {
	return complete.PredictAnything
}

func (c *Command) AutocompleteFlags() complete.Flags {
	return c.Flags().Completions()
}

func (c *Command) Run(args []string) int {
	if os.Getenv("BOUNDARY_EXAMPLE_CLI_OUTPUT") != "" {
		c.UI.Output(exampleOutput())
		return 0
	}

	switch c.Func {
	case "", "create", "update":
		return cli.RunResultHelp
	}

	f := c.Flags()

	if err := f.Parse(args); err != nil {
		c.UI.Error(err.Error())
		return 1
	}

	if strutil.StrListContains(flagsMap[c.Func], "id") && c.FlagId == "" {
		c.UI.Error("ID is required but not passed in via -id")
		return 1
	}
	if strutil.StrListContains(flagsMap[c.Func], "scope-id") && c.FlagScopeId == "" {
		c.UI.Error("Scope ID must be passed in via -scope-id")
		return 1
	}

	client, err := c.Client()
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error creating API client: %s", err.Error()))
		return 2
	}

	var opts []targets.Option

	switch c.FlagName {
	case "":
	case "null":
		opts = append(opts, targets.DefaultName())
	default:
		opts = append(opts, targets.WithName(c.FlagName))
	}

	switch c.FlagDescription {
	case "":
	case "null":
		opts = append(opts, targets.DefaultDescription())
	default:
		opts = append(opts, targets.WithDescription(c.FlagDescription))
	}

	hostSets := c.flagHostSets
	switch c.Func {
	case "add-host-sets", "remove-host-sets":
		if len(c.flagHostSets) == 0 {
			c.UI.Error("No host-sets supplied via -host-set")
			return 1
		}

	case "set-host-sets":
		switch len(c.flagHostSets) {
		case 0:
			c.UI.Error("No host-sets supplied via -host-set")
			return 1
		case 1:
			if c.flagHostSets[0] == "null" {
				hostSets = nil
			}
		}
	case "authorize":
		if len(c.flagHostId) != 0 {
			opts = append(opts, targets.WithHostId(c.flagHostId))
		}
	}

	// Perform check-and-set when needed
	var version uint32
	switch c.Func {
	case "add-host-sets", "remove-host-sets", "set-host-sets":
		switch c.FlagVersion {
		case 0:
			opts = append(opts, targets.WithAutomaticVersioning(true))
		default:
			version = uint32(c.FlagVersion)
		}
	default:
		// The only other one that needs it is update, handled by the static
		// file
	}

	targetClient := targets.NewClient(client)

	existed := true
	var result api.GenericResult
	var listResult api.GenericListResult
	var apiErr *api.Error
	var sar *targets.SessionAuthorizationResult

	switch c.Func {
	case "read":
		result, apiErr, err = targetClient.Read(c.Context, c.FlagId, opts...)
	case "delete":
		_, apiErr, err = targetClient.Delete(c.Context, c.FlagId, opts...)
		if apiErr != nil && apiErr.Status == int32(http.StatusNotFound) {
			existed = false
			apiErr = nil
		}
	case "list":
		listResult, apiErr, err = targetClient.List(c.Context, c.FlagScopeId, opts...)
	case "add-host-sets":
		result, apiErr, err = targetClient.AddHostSets(c.Context, c.FlagId, version, hostSets, opts...)
	case "remove-host-sets":
		result, apiErr, err = targetClient.RemoveHostSets(c.Context, c.FlagId, version, hostSets, opts...)
	case "set-host-sets":
		result, apiErr, err = targetClient.SetHostSets(c.Context, c.FlagId, version, hostSets, opts...)
	case "authorize":
		sar, apiErr, err = targetClient.Authorize(c.Context, c.FlagId, opts...)
	}

	plural := "target"
	switch c.Func {
	case "list":
		plural = "targets"
	case "authorize":
		plural = "a session against target"
	}
	if err != nil {
		c.UI.Error(fmt.Sprintf("Error trying to %s %s: %s", c.Func, plural, err.Error()))
		return 2
	}
	if apiErr != nil {
		c.UI.Error(fmt.Sprintf("Error from controller when performing %s on %s: %s", c.Func, plural, pretty.Sprint(apiErr)))
		return 1
	}

	switch c.Func {
	case "delete":
		switch base.Format(c.UI) {
		case "json":
			c.UI.Output("null")
		case "table":
			output := "The delete operation completed successfully"
			switch existed {
			case true:
				output += "."
			default:
				output += ", however the resource did not exist at the time."
			}
			c.UI.Output(output)
		}
		return 0

	case "list":
		listedTargets := listResult.GetItems().([]*targets.Target)
		switch base.Format(c.UI) {
		case "json":
			if len(listedTargets) == 0 {
				c.UI.Output("null")
				return 0
			}
			b, err := base.JsonFormatter{}.Format(listedTargets)
			if err != nil {
				c.UI.Error(fmt.Errorf("Error formatting as JSON: %w", err).Error())
				return 1
			}
			c.UI.Output(string(b))

		case "table":
			if len(listedTargets) == 0 {
				c.UI.Output("No targets found")
				return 0
			}
			var output []string
			output = []string{
				"",
				"Target information:",
			}
			for i, m := range listedTargets {
				if i > 0 {
					output = append(output, "")
				}
				if true {
					output = append(output,
						fmt.Sprintf("  ID:             %s", m.Id),
						fmt.Sprintf("    Version:      %d", m.Version),
						fmt.Sprintf("    Type:         %s", m.Type),
					)
				}
				if m.Name != "" {
					output = append(output,
						fmt.Sprintf("    Name:         %s", m.Name),
					)
				}
				if m.Description != "" {
					output = append(output,
						fmt.Sprintf("    Description:  %s", m.Description),
					)
				}
			}
			c.UI.Output(base.WrapForHelpText(output))
		}
		return 0

	case "authorize":
		sa := sar.GetItem().(*targets.SessionAuthorization)
		switch base.Format(c.UI) {
		case "table":
			c.UI.Output(generateAuthorizationTableOutput(sa))
		case "json":
			b, err := base.JsonFormatter{}.Format(sa)
			if err != nil {
				c.UI.Error(fmt.Errorf("Error formatting as JSON: %w", err).Error())
				return 1
			}
			c.UI.Output(string(b))
		}
		return 0
	}

	target := result.GetItem().(*targets.Target)
	switch base.Format(c.UI) {
	case "table":
		c.UI.Output(generateTargetTableOutput(target))
	case "json":
		b, err := base.JsonFormatter{}.Format(target)
		if err != nil {
			c.UI.Error(fmt.Errorf("Error formatting as JSON: %w", err).Error())
			return 1
		}
		c.UI.Output(string(b))
	}

	return 0
}