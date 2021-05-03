package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/influxdata/influxdb/v2/kit/platform"

	"github.com/fatih/color"
	"github.com/influxdata/influxdb/v2"
	"github.com/influxdata/influxdb/v2/cmd/influx/internal"
	internal2 "github.com/influxdata/influxdb/v2/cmd/internal"
	ierror "github.com/influxdata/influxdb/v2/kit/errors"
	"github.com/influxdata/influxdb/v2/pkger"
	"github.com/influxdata/influxdb/v2/tenant"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	input "github.com/tcnksm/go-input"
)

type templateSVCsFn func() (pkger.SVC, influxdb.OrganizationService, error)

func cmdApply(f *globalFlags, opts genericCLIOpts) *cobra.Command {
	return newCmdPkgerBuilder(newPkgerSVC, f, opts).cmdApply()
}

func cmdExport(f *globalFlags, opts genericCLIOpts) *cobra.Command {
	return newCmdPkgerBuilder(newPkgerSVC, f, opts).cmdExport()
}

func cmdStack(f *globalFlags, opts genericCLIOpts) *cobra.Command {
	return newCmdPkgerBuilder(newPkgerSVC, f, opts).cmdStacks()
}

func cmdTemplate(f *globalFlags, opts genericCLIOpts) *cobra.Command {
	return newCmdPkgerBuilder(newPkgerSVC, f, opts).cmdTemplate()
}

type cmdTemplateBuilder struct {
	genericCLIOpts
	*globalFlags

	svcFn templateSVCsFn

	encoding            string
	file                string
	files               []string
	filters             []string
	description         string
	disableColor        bool
	disableTableBorders bool
	hideHeaders         bool
	json                bool
	force               bool
	name                string
	names               []string
	org                 organization
	quiet               bool
	recurse             bool
	stackID             string
	stackIDs            []string
	urls                []string

	applyOpts struct {
		envRefs []string
		force   string
		secrets []string
	}

	exportOpts struct {
		resourceType   string
		buckets        string
		checks         string
		dashboards     string
		endpoints      string
		labels         string
		rules          string
		tasks          string
		telegrafs      string
		variables      string
		bucketNames    string
		checkNames     string
		dashboardNames string
		endpointNames  string
		labelNames     string
		ruleNames      string
		taskNames      string
		telegrafNames  string
		variableNames  string
	}

	updateStackOpts struct {
		addResources []string
	}
}

func newCmdPkgerBuilder(svcFn templateSVCsFn, f *globalFlags, opts genericCLIOpts) *cmdTemplateBuilder {
	return &cmdTemplateBuilder{
		genericCLIOpts: opts,
		globalFlags:    f,
		svcFn:          svcFn,
	}
}

func (b *cmdTemplateBuilder) cmdApply() *cobra.Command {
	cmd := b.newCmd("apply", b.applyRunEFn)
	cmd.Short = "Apply a template to manage resources"
	cmd.Long = `
	The apply command applies InfluxDB template(s). Use the command to create new
	resources via a declarative template. The apply command can consume templates
	via file(s), url(s), stdin, or any combination of the 3. Each run of the apply
	command ensures that all templates applied are applied in unison as a transaction.
	If any unexpected errors are discovered then all side effects are rolled back.

	Examples:
		# Apply a template via a file
		influx apply -f $PATH_TO_TEMPLATE/template.json

		# Apply a stack that has associated templates. In this example the stack has a remote
		# template associated with it.
		influx apply --stack-id $STACK_ID

		# Apply a template associated with a stack. Stacks make template application idempotent.
		influx apply -f $PATH_TO_TEMPLATE/template.json --stack-id $STACK_ID

		# Apply multiple template files together (mix of yaml and json)
		influx apply \
			-f $PATH_TO_TEMPLATE/template_1.json \
			-f $PATH_TO_TEMPLATE/template_2.yml

		# Apply a template from a url
		influx apply -f https://raw.githubusercontent.com/influxdata/community-templates/master/docker/docker.yml

		# Apply a template from STDIN
		cat $TEMPLATE.json | influx apply --encoding json

		# Applying a directory of templates, takes everything from provided directory
		influx apply -f $PATH_TO_TEMPLATE_DIR

		# Applying a directory of templates, recursively descending into child directories
		influx apply -R -f $PATH_TO_TEMPLATE_DIR

		# Applying directories from many sources, file and URL
		influx apply -f $PATH_TO_TEMPLATE/template.yml -f $URL_TO_TEMPLATE

		# Applying a template with actions to skip resources applied. The
		# following example skips all buckets and the dashboard whose 
		# metadata.name field matches the provided $DASHBOARD_TMPL_NAME.
		# format for filters:
		#	--filter=kind=Bucket
		#	--filter=resource=Label:$Label_TMPL_NAME
		influx apply \
			-f $PATH_TO_TEMPLATE/template.yml \
			--filter kind=Bucket \
			--filter resource=Dashboard:$DASHBOARD_TMPL_NAME

	For information about finding and using InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/apply/.

	For more templates created by the community, see
	https://github.com/influxdata/community-templates.
`

	b.org.register(b.viper, cmd, false)
	b.registerTemplateFileFlags(cmd)
	b.registerTemplatePrintOpts(cmd)
	cmd.Flags().BoolVarP(&b.quiet, "quiet", "q", false, "Disable output printing")
	cmd.Flags().StringVar(&b.applyOpts.force, "force", "", `TTY input, if template will have destructive changes, proceed if set "true"`)
	cmd.Flags().StringVar(&b.stackID, "stack-id", "", "Stack ID to associate template application")

	b.applyOpts.secrets = []string{}
	cmd.Flags().StringSliceVar(&b.applyOpts.secrets, "secret", nil, "Secrets to provide alongside the template; format should --secret=SECRET_KEY=SECRET_VALUE --secret=SECRET_KEY_2=SECRET_VALUE_2")
	cmd.Flags().StringSliceVar(&b.applyOpts.envRefs, "env-ref", nil, "Environment references to provide alongside the template; format should --env-ref=REF_KEY=REF_VALUE --env-ref=REF_KEY_2=REF_VALUE_2")
	cmd.Flags().StringSliceVar(&b.filters, "filter", nil, "Resources to skip when applying the template. Filter out by ‘kind’ or by ‘resource’")

	return cmd
}

func (b *cmdTemplateBuilder) applyRunEFn(cmd *cobra.Command, args []string) error {
	if err := b.org.validOrgFlags(&flags); err != nil {
		return err
	}
	color.NoColor = b.disableColor

	svc, orgSVC, err := b.svcFn()
	if err != nil {
		return err
	}

	influxOrgID, err := b.org.getID(orgSVC)
	if err != nil {
		return err
	}

	template, isTTY, err := b.readTemplate()
	if err != nil {
		return err
	}

	providedEnvRefs := mapKeys(template.Summary().MissingEnvs, b.applyOpts.envRefs)
	if !isTTY {
		for _, envRef := range missingValKeys(providedEnvRefs) {
			prompt := "Please provide environment reference value for key " + envRef
			providedEnvRefs[envRef] = b.getInput(prompt, "")
		}
	}

	var stackID platform.ID
	if b.stackID != "" {
		if err := stackID.DecodeFromString(b.stackID); err != nil {
			return err
		}
	}

	opts := []pkger.ApplyOptFn{
		pkger.ApplyWithTemplate(template),
		pkger.ApplyWithEnvRefs(toMapInterface(providedEnvRefs)),
		pkger.ApplyWithStackID(stackID),
	}

	actionOpts, err := parseTemplateActions(b.filters)
	if err != nil {
		return err
	}
	opts = append(opts, actionOpts...)

	dryRunImpact, err := svc.DryRun(context.Background(), influxOrgID, 0, opts...)
	if err != nil {
		return err
	}

	providedSecrets := mapKeys(dryRunImpact.Summary.MissingSecrets, b.applyOpts.secrets)
	if !isTTY {
		const skipDefault = "$$skip-this-key$$"
		for _, secretKey := range missingValKeys(providedSecrets) {
			prompt := "Please provide secret value for key " + secretKey + " (optional, press enter to skip)"
			secretVal := b.getInput(prompt, skipDefault)
			if secretVal != "" && secretVal != skipDefault {
				providedSecrets[secretKey] = secretVal
			}
		}
	}

	if err := b.printTemplateDiff(dryRunImpact.Diff); err != nil {
		return err
	}

	isForced, _ := strconv.ParseBool(b.applyOpts.force)
	if !isTTY && !isForced && b.applyOpts.force != "conflict" {
		confirm := b.getInput("Confirm application of the above resources (y/n)", "n")
		if strings.ToLower(confirm) != "y" {
			fmt.Fprintln(b.w, "aborted application of template")
			return nil
		}
	}

	if b.applyOpts.force != "conflict" && isTTY && dryRunImpact.Diff.HasConflicts() {
		return errors.New("template has conflicts with existing resources and cannot safely apply")
	}

	opts = append(opts, pkger.ApplyWithSecrets(providedSecrets))

	impact, err := svc.Apply(context.Background(), influxOrgID, 0, opts...)
	if err != nil {
		return err
	}

	b.printTemplateSummary(impact.StackID, impact.Summary)

	return nil
}

func parseTemplateActions(args []string) ([]pkger.ApplyOptFn, error) {
	var opts []pkger.ApplyOptFn
	for _, rawAct := range args {
		pair := strings.SplitN(rawAct, "=", 2)
		if len(pair) < 2 {
			continue
		}
		key, val := pair[0], pair[1]
		switch strings.ToLower(key) {
		case "kind":
			opts = append(opts, pkger.ApplyWithKindSkip(pkger.ActionSkipKind{
				Kind: pkger.Kind(val),
			}))
		case "resource":
			pp := strings.SplitN(val, ":", 2)
			if len(pair) != 2 {
				return nil, fmt.Errorf(`invalid skipResource action provided: %q; 
	Expected format --action=skipResource=Label:$LABEL_ID`, rawAct)
			}
			kind, metaName := pp[0], pp[1]
			opts = append(opts, pkger.ApplyWithResourceSkip(pkger.ActionSkipResource{
				Kind:     pkger.Kind(kind),
				MetaName: metaName,
			}))
		default:
			return nil, fmt.Errorf(`invalid action provided: %q; 
	Expected format --action=skipResource=Label:$LABEL_ID
	or
	Expected format --action=skipKind=Bucket`, rawAct)
		}
	}

	return opts, nil
}

func (b *cmdTemplateBuilder) cmdExport() *cobra.Command {
	cmd := b.newCmd("export", b.exportRunEFn)
	cmd.Short = "Export existing resources as a template"
	cmd.Long = `
	The export command provides a mechanism to export existing resources to a
	template. Each template resource kind is supported via flags.

	Examples:
		# export buckets by ID
		influx export --buckets=$ID1,$ID2,$ID3

		# export buckets, labels, and dashboards by ID
		influx export \
			--buckets=$BID1,$BID2,$BID3 \
			--labels=$LID1,$LID2,$LID3 \
			--dashboards=$DID1,$DID2,$DID3

		# export all resources for a stack
		influx export --stack-id $STACK_ID

		# export a stack with resources not associated with the stack
		influx export --stack-id $STACK_ID --buckets $BUCKET_ID

	All of the resources are supported via the examples provided above. Provide the
	resource flag and then provide the IDs.

	For information about exporting InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/export/
`
	cmd.AddCommand(
		b.cmdExportAll(),
		b.cmdExportStack(),
	)

	cmd.Flags().StringVarP(&b.file, "file", "f", "", "Output file for created template; defaults to std out if no file provided; the extension of provided file (.yml/.json) will dictate encoding")
	cmd.Flags().StringVar(&b.stackID, "stack-id", "", "ID for stack to include in export")
	cmd.Flags().StringVar(&b.exportOpts.resourceType, "resource-type", "", "Resource type provided will be associated with all IDs via stdin.")
	cmd.Flags().StringVar(&b.exportOpts.buckets, "buckets", "", "List of bucket ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.checks, "checks", "", "List of check ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.dashboards, "dashboards", "", "List of dashboard ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.endpoints, "endpoints", "", "List of notification endpoint ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.labels, "labels", "", "List of label ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.rules, "rules", "", "List of notification rule ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.tasks, "tasks", "", "List of task ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.telegrafs, "telegraf-configs", "", "List of telegraf config ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.variables, "variables", "", "List of variable ids comma separated")
	cmd.Flags().StringVar(&b.exportOpts.bucketNames, "bucket-names", "", "List of bucket names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.checkNames, "check-names", "", "List of check names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.dashboardNames, "dashboard-names", "", "List of dashboard names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.endpointNames, "endpoint-names", "", "List of notification endpoint names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.labelNames, "label-names", "", "List of label names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.ruleNames, "rule-names", "", "List of notification rule names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.taskNames, "task-names", "", "List of task names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.telegrafNames, "telegraf-config-names", "", "List of telegraf config names comma separated")
	cmd.Flags().StringVar(&b.exportOpts.variableNames, "variable-names", "", "List of variable names comma separated")

	return cmd
}

func (b *cmdTemplateBuilder) exportRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, _, err := b.svcFn()
	if err != nil {
		return err
	}

	resTypes := []struct {
		kind   pkger.Kind
		idStrs []string
		names  []string
	}{
		{kind: pkger.KindBucket, idStrs: strings.Split(b.exportOpts.buckets, ","), names: strings.Split(b.exportOpts.bucketNames, ",")},
		{kind: pkger.KindCheck, idStrs: strings.Split(b.exportOpts.checks, ","), names: strings.Split(b.exportOpts.checkNames, ",")},
		{kind: pkger.KindDashboard, idStrs: strings.Split(b.exportOpts.dashboards, ","), names: strings.Split(b.exportOpts.dashboardNames, ",")},
		{kind: pkger.KindLabel, idStrs: strings.Split(b.exportOpts.labels, ","), names: strings.Split(b.exportOpts.labelNames, ",")},
		{kind: pkger.KindNotificationEndpoint, idStrs: strings.Split(b.exportOpts.endpoints, ","), names: strings.Split(b.exportOpts.endpointNames, ",")},
		{kind: pkger.KindNotificationRule, idStrs: strings.Split(b.exportOpts.rules, ","), names: strings.Split(b.exportOpts.ruleNames, ",")},
		{kind: pkger.KindTask, idStrs: strings.Split(b.exportOpts.tasks, ","), names: strings.Split(b.exportOpts.taskNames, ",")},
		{kind: pkger.KindTelegraf, idStrs: strings.Split(b.exportOpts.telegrafs, ","), names: strings.Split(b.exportOpts.telegrafNames, ",")},
		{kind: pkger.KindVariable, idStrs: strings.Split(b.exportOpts.variables, ","), names: strings.Split(b.exportOpts.variableNames, ",")},
	}

	var opts []pkger.ExportOptFn
	for _, rt := range resTypes {
		newOpt, err := newResourcesToClone(rt.kind, rt.idStrs, rt.names)
		if err != nil {
			return ierror.Wrap(err, rt.kind.String())
		}
		opts = append(opts, newOpt)
	}

	if b.stackID != "" {
		stackID, err := platform.IDFromString(b.stackID)
		if err != nil {
			return ierror.Wrap(err, "invalid stack ID provided")
		}
		opts = append(opts, pkger.ExportWithStackID(*stackID))
	}

	if b.exportOpts.resourceType == "" {
		return b.exportTemplate(cmd.OutOrStdout(), templateSVC, b.file, opts...)
	}

	resKind := templateKindFold(b.exportOpts.resourceType)

	if err := resKind.OK(); err != nil {
		return errors.New("resource type is invalid; got: " + b.exportOpts.resourceType)
	}

	if stdin, err := inStdIn(b.in); err == nil {
		stdinInpt, _ := b.readLines(stdin)
		if len(stdinInpt) > 0 {
			args = stdinInpt
		}
	}

	resTypeOpt, err := newResourcesToClone(resKind, args, []string{})
	if err != nil {
		return err
	}
	opts = append(opts, resTypeOpt)

	return b.exportTemplate(cmd.OutOrStdout(), templateSVC, b.file, opts...)
}

func (b *cmdTemplateBuilder) cmdExportAll() *cobra.Command {
	cmd := b.newCmd("all", b.exportAllRunEFn)
	cmd.Short = "Export all existing resources for an organization as a template"
	cmd.Long = `
	The export all command will export all resources for an organization. The
	command also provides a mechanism to filter by label name or resource kind.

	Examples:
		# Export all resources for an organization
		influx export all --org $ORG_NAME

		# Export all bucket resources
		influx export all --org $ORG_NAME --filter=kind=Bucket

		# Export all resources associated with label Foo
		influx export all --org $ORG_NAME --filter=labelName=Foo

		# Export all bucket resources and filter by label Foo
		influx export all --org $ORG_NAME \
			--filter=kind=Bucket \
			--filter=labelName=Foo

		# Export all bucket or dashboard resources and filter by label Foo.
		# note: like filters are unioned and filter types are intersections.
		#		This example will export a resource if it is a dashboard or
		#		bucket and has an associated label of Foo.
		influx export all --org $ORG_NAME \
			--filter=kind=Bucket \
			--filter=kind=Dashboard \
			--filter=labelName=Foo

	For information about exporting InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/export/
	and
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/export/all/
`

	cmd.Flags().StringVarP(&b.file, "file", "f", "", "output file for created template; defaults to std out if no file provided; the extension of provided file (.yml/.json) will dictate encoding")
	cmd.Flags().StringArrayVar(&b.filters, "filter", nil, "Filter exported resources by labelName or resourceKind (format: --filter=labelName=example)")

	b.org.register(b.viper, cmd, false)

	return cmd
}

func (b *cmdTemplateBuilder) exportAllRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, orgSVC, err := b.svcFn()
	if err != nil {
		return err
	}

	orgID, err := b.org.getID(orgSVC)
	if err != nil {
		return err
	}

	var (
		labelNames    []string
		resourceKinds []pkger.Kind
	)
	for _, filter := range b.filters {
		pair := strings.SplitN(filter, "=", 2)
		if len(pair) < 2 {
			continue
		}
		switch key, val := pair[0], pair[1]; key {
		case "labelName":
			labelNames = append(labelNames, val)
		case "kind", "resourceKind":
			k := pkger.Kind(val)
			if err := k.OK(); err != nil {
				return err
			}
			resourceKinds = append(resourceKinds, k)
		default:
			return fmt.Errorf("invalid filter provided %q; filter must be 1 in [labelName, resourceKind]", filter)
		}
	}

	orgOpt := pkger.ExportWithAllOrgResources(pkger.ExportByOrgIDOpt{
		OrgID:         orgID,
		LabelNames:    labelNames,
		ResourceKinds: resourceKinds,
	})
	return b.exportTemplate(cmd.OutOrStdout(), templateSVC, b.file, orgOpt)
}

func (b *cmdTemplateBuilder) cmdExportStack() *cobra.Command {
	cmd := b.newCmd("stack $STACK_ID", b.exportStackRunEFn)
	cmd.Short = "Export all existing resources for associated with a stack as a template"
	cmd.Long = `
	The export stack command exports the resources associated with a stack as
	they currently exist in the platform. All the same metadata.name fields will be
	reused.

	Example:
		# Export by a stack
		influx export stack $STACK_ID

	For information about exporting InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/export/
	and
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/export/stack/
`
	cmd.Args = cobra.ExactValidArgs(1)

	cmd.Flags().StringVarP(&b.file, "file", "f", "", "output file for created template; defaults to std out if no file provided; the extension of provided file (.yml/.json) will dictate encoding")
	b.org.register(b.viper, cmd, false)

	return cmd
}

func (b *cmdTemplateBuilder) exportStackRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, _, err := b.svcFn()
	if err != nil {
		return err
	}

	stackID, err := platform.IDFromString(args[0])
	if err != nil {
		return err
	}

	template, err := templateSVC.Export(context.Background(), pkger.ExportWithStackID(*stackID))
	if err != nil {
		return err
	}

	return b.writeTemplate(b.w, b.file, template)
}

func (b *cmdTemplateBuilder) cmdTemplate() *cobra.Command {
	runE := func(cmd *cobra.Command, args []string) error {
		template, _, err := b.readTemplate()
		if err != nil {
			return err
		}

		return b.printTemplateSummary(0, template.Summary())
	}

	cmd := b.genericCLIOpts.newCmd("template", runE, false)

	b.registerTemplateFileFlags(cmd)
	b.registerTemplatePrintOpts(cmd)
	cmd.Short = "Summarize the provided template"

	cmd.AddCommand(b.cmdTemplateValidate())
	return cmd
}

func (b *cmdTemplateBuilder) cmdTemplateValidate() *cobra.Command {
	runE := func(cmd *cobra.Command, args []string) error {
		template, _, err := b.readTemplate()
		if err != nil {
			return err
		}
		return template.Validate()
	}

	cmd := b.genericCLIOpts.newCmd("validate", runE, false)
	cmd.Short = "Validate the provided template"

	b.registerTemplateFileFlags(cmd)

	return cmd
}

func (b *cmdTemplateBuilder) cmdStacks() *cobra.Command {
	cmd := b.newCmd("stacks [flags]", b.stackListRunEFn)
	cmd.Flags().StringArrayVar(&b.stackIDs, "stack-id", nil, "Stack ID to filter by")
	cmd.Flags().StringArrayVar(&b.names, "stack-name", nil, "Stack name to filter by")
	registerPrintOptions(b.viper, cmd, &b.hideHeaders, &b.json)

	b.org.register(b.viper, cmd, false)

	cmd.Short = "List stack(s) and associated templates. Subcommands manage stacks."
	cmd.Long = `
	List stack(s) and associated templates. Subcommands manage stacks.

	Examples:
		# list all known stacks
		influx stacks

		# list stacks filtered by stack name
		# output here are stacks that have match at least 1 name provided
		influx stacks --stack-name=$STACK_NAME_1 --stack-name=$STACK_NAME_2

		# list stacks filtered by stack id
		# output here are stacks that have match at least 1 ids provided
		influx stacks --stack-id=$STACK_ID_1 --stack-id=$STACK_ID_2
		
		# list stacks filtered by stack id or stack name
		# output here are stacks that have match the id provided or
		# matches of the name provided
		influx stacks --stack-id=$STACK_ID --stack-name=$STACK_NAME

	For information about Stacks and how they integrate with InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/stacks/
`

	cmd.AddCommand(
		b.cmdStackInit(),
		b.cmdStackRemove(),
		b.cmdStackUpdate(),
	)
	return cmd
}

func (b *cmdTemplateBuilder) cmdStackInit() *cobra.Command {
	cmd := b.newCmd("init", b.stackInitRunEFn)
	cmd.Short = "Initialize a stack"
	cmd.Long = `
	The stack init command creates a new stack to associated templates with. A
	stack is used to make applying templates idempotent. When you apply a template
	and associate it with a stack, the stack can manage the created/updated resources
	from the template back to the platform. This enables a multitude of useful features.
	Any associated template urls will be applied when applying templates via a stack.

	Examples:
		# Initialize a stack with a name and description
		influx stacks init -n $STACK_NAME -d $STACK_DESCRIPTION

		# Initialize a stack with a name and urls to associate with stack.
		influx stacks init -n $STACK_NAME -u $PATH_TO_TEMPLATE

	For information about how stacks work with InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/stacks/
	and
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/stacks/init/
`

	cmd.Flags().StringVarP(&b.name, "stack-name", "n", "", "Name given to created stack")
	cmd.Flags().StringVarP(&b.description, "stack-description", "d", "", "Description given to created stack")
	cmd.Flags().StringArrayVarP(&b.urls, "template-url", "u", nil, "Template urls to associate with new stack")
	registerPrintOptions(b.viper, cmd, &b.hideHeaders, &b.json)

	b.org.register(b.viper, cmd, false)

	return cmd
}

func (b *cmdTemplateBuilder) stackInitRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, orgSVC, err := b.svcFn()
	if err != nil {
		return err
	}

	orgID, err := b.org.getID(orgSVC)
	if err != nil {
		return err
	}

	const fakeUserID = 0 // is 0 because user is pulled from token...
	stack, err := templateSVC.InitStack(context.Background(), fakeUserID, pkger.StackCreate{
		OrgID:        orgID,
		Name:         b.name,
		Description:  b.description,
		TemplateURLs: b.urls,
	})
	if err != nil {
		return err
	}

	return b.writeStack(stack)
}

func (b *cmdTemplateBuilder) stackListRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, orgSVC, err := b.svcFn()
	if err != nil {
		return err
	}

	orgID, err := b.org.getID(orgSVC)
	if err != nil {
		return err
	}

	var stackIDs []platform.ID
	for _, rawID := range b.stackIDs {
		id, err := platform.IDFromString(rawID)
		if err != nil {
			return err
		}
		stackIDs = append(stackIDs, *id)
	}

	stacks, err := templateSVC.ListStacks(context.Background(), orgID, pkger.ListFilter{
		StackIDs: stackIDs,
		Names:    b.names,
	})
	if err != nil {
		return err
	}

	if b.json {
		return b.writeJSON(stacks)
	}

	tabW := b.newTabWriter()
	defer tabW.Flush()

	tabW.HideHeaders(b.hideHeaders)
	writeStackRows(tabW, stacks...)

	return nil
}

func (b *cmdTemplateBuilder) cmdStackRemove() *cobra.Command {
	cmd := b.newCmd("rm [--stack-id=ID1 --stack-id=ID2]", b.stackRemoveRunEFn)
	cmd.Short = "Remove a stack(s) and all associated resources"
	cmd.Aliases = []string{"remove", "uninstall"}

	cmd.Flags().StringArrayVar(&b.stackIDs, "stack-id", nil, "Stack IDs to be removed")
	cmd.Flags().BoolVar(&b.force, "force", false, "Remove stack without confirmation prompt")
	cmd.MarkFlagRequired("stack-id")
	registerPrintOptions(b.viper, cmd, &b.hideHeaders, &b.json)

	b.org.register(b.viper, cmd, false)

	return cmd
}

func (b *cmdTemplateBuilder) stackRemoveRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, orgSVC, err := b.svcFn()
	if err != nil {
		return err
	}

	orgID, err := b.org.getID(orgSVC)
	if err != nil {
		return err
	}

	var stackIDs []platform.ID
	for _, rawID := range b.stackIDs {
		id, err := platform.IDFromString(rawID)
		if err != nil {
			return err
		}
		stackIDs = append(stackIDs, *id)
	}

	stacks, err := templateSVC.ListStacks(context.Background(), orgID, pkger.ListFilter{
		StackIDs: stackIDs,
	})
	if err != nil {
		return err
	}

	printStack := func(stack pkger.Stack) error {
		if b.json {
			return b.writeJSON(stack)
		}

		tabW := b.newTabWriter()
		defer func() {
			tabW.Flush()
			// add a breather line between confirm and printout
			fmt.Fprintln(b.w)
		}()

		tabW.HideHeaders(b.hideHeaders)
		writeStackRows(tabW, stack)
		return nil
	}

	for _, stack := range stacks {
		if err := printStack(stack); err != nil {
			return err
		}

		if !b.force {
			msg := fmt.Sprintf("Confirm removal of the stack[%s] and all associated resources (y/n)", stack.ID)
			confirm := b.getInput(msg, "n")
			if confirm != "y" {
				continue
			}
		}

		err := templateSVC.DeleteStack(context.Background(), struct{ OrgID, UserID, StackID platform.ID }{
			OrgID:   orgID,
			UserID:  0,
			StackID: stack.ID,
		})
		if err != nil {
			return err
		}
	}

	return nil
}

func (b *cmdTemplateBuilder) cmdStackUpdate() *cobra.Command {
	cmd := b.newCmd("update", b.stackUpdateRunEFn)
	cmd.Short = "Update a stack"
	cmd.Long = `
	The stack update command updates a stack.

	Examples:
		# Update a stack with a name and description
		influx stacks update -i $STACK_ID -n $STACK_NAME -d $STACK_DESCRIPTION

		# Update a stack with a name and urls to associate with stack.
		influx stacks update --stack-id $STACK_ID --stack-name $STACK_NAME --template-url $PATH_TO_TEMPLATE

		# Update stack with new resources to manage
		influx stacks update \
			--stack-id $STACK_ID \
			--addResource=Bucket=$BUCKET_ID \
			--addResource=Dashboard=$DASH_ID

		# Update stack with new resources to manage and export stack
		# as a template
		influx stacks update \
			--stack-id $STACK_ID \
			--addResource=Bucket=$BUCKET_ID \
			--export-file /path/to/file.yml

	For information about how stacks work with InfluxDB templates, see
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/stacks/
	and
	https://docs.influxdata.com/influxdb/latest/reference/cli/influx/stacks/update/
`

	cmd.Flags().StringVarP(&b.stackID, "stack-id", "i", "", "ID of stack")
	cmd.MarkFlagRequired("stack-id")
	cmd.Flags().StringVarP(&b.name, "stack-name", "n", "", "Name for stack")
	cmd.Flags().StringVarP(&b.description, "stack-description", "d", "", "Description for stack")
	cmd.Flags().StringArrayVarP(&b.urls, "template-url", "u", nil, "Template urls to associate with stack")
	cmd.Flags().StringArrayVar(&b.updateStackOpts.addResources, "addResource", nil, "Additional resources to associate with stack")
	cmd.Flags().StringVarP(&b.file, "export-file", "f", "", "Destination for exported template")
	registerPrintOptions(b.viper, cmd, &b.hideHeaders, &b.json)

	return cmd
}

func (b *cmdTemplateBuilder) stackUpdateRunEFn(cmd *cobra.Command, args []string) error {
	templateSVC, _, err := b.svcFn()
	if err != nil {
		return err
	}

	stackID, err := platform.IDFromString(b.stackID)
	if err != nil {
		return ierror.Wrap(err, "required stack id is invalid")
	}

	var name, description *string

	if cmd.Flags().Lookup("stack-name").Changed {
		name = &b.name
	}
	if cmd.Flags().Lookup("stack-description").Changed {
		description = &b.description
	}

	update := pkger.StackUpdate{
		ID:           *stackID,
		Name:         name,
		Description:  description,
		TemplateURLs: b.urls,
	}

	failedString := []string{}
	for _, res := range b.updateStackOpts.addResources {
		parts := strings.SplitN(res, "=", 2)
		if len(parts) < 2 {
			failedString = append(failedString, res)
			continue
		}

		kind, idRaw := templateKindFold(parts[0]), parts[1]
		if err := kind.OK(); err != nil {
			return errors.New("resource type is invalid; got: " + b.exportOpts.resourceType)
		}

		id, err := platform.IDFromString(idRaw)
		if err != nil {
			return ierror.Wrap(err, fmt.Sprintf("%s resource id %q is invalid", kind, idRaw))
		}
		update.AdditionalResources = append(update.AdditionalResources, pkger.StackAdditionalResource{
			APIVersion: pkger.APIVersion,
			ID:         *id,
			Kind:       kind,
		})
	}

	if len(failedString) > 0 {
		return errors.New("invalid 'resourceType=resourceID' key value pair[s]: " + strings.Join(failedString, ";"))
	}

	stack, err := templateSVC.UpdateStack(context.Background(), update)
	if err != nil {
		return err
	}

	if err := b.writeStack(stack); err != nil {
		return err
	}

	if len(update.AdditionalResources) == 0 {
		return nil
	}

	if b.file == "" {
		const msg = `
Your stack now differs from your template. Applying an outdated template will revert
these updates. Export a new template with these updates to prevent accidental changes? (y/n)`
		for range make([]struct{}, 3) {
			if in := b.getInput(msg, "n"); in == "y" {
				break
			} else if in == "n" {
				return nil
			}
		}
	}

	return b.exportTemplate(cmd.OutOrStdout(), templateSVC, b.file, pkger.ExportWithStackID(*stackID))
}

func (b *cmdTemplateBuilder) writeStack(stack pkger.Stack) error {
	if b.json {
		return b.writeJSON(stack)
	}

	tabW := b.newTabWriter()
	defer tabW.Flush()

	tabW.HideHeaders(b.hideHeaders)
	writeStackRows(tabW, stack)

	return nil
}

func (b *cmdTemplateBuilder) newCmd(use string, runE func(*cobra.Command, []string) error) *cobra.Command {
	cmd := b.genericCLIOpts.newCmd(use, runE, true)
	b.globalFlags.registerFlags(b.viper, cmd)
	return cmd
}

func (b *cmdTemplateBuilder) registerTemplatePrintOpts(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&b.disableColor, "disable-color", false, "Disable color in output")
	cmd.Flags().BoolVar(&b.disableTableBorders, "disable-table-borders", false, "Disable table borders")
	registerPrintOptions(b.viper, cmd, nil, &b.json)
}

func (b *cmdTemplateBuilder) registerTemplateFileFlags(cmd *cobra.Command) {
	cmd.Flags().StringSliceVarP(&b.files, "file", "f", nil, "Path to template file; Supports HTTP(S) URLs or file paths.")
	cmd.MarkFlagFilename("file", "yaml", "yml", "json", "jsonnet")
	cmd.Flags().BoolVarP(&b.recurse, "recurse", "R", false, "Process the directory used in -f, --file recursively. Useful when you want to manage related templates organized within the same directory.")

	cmd.Flags().StringSliceVarP(&b.urls, "template-url", "u", nil, "URL to template file")
	cmd.Flags().MarkDeprecated("template-url", "use the --file flag; example: influx apply --file $URL_TO_TEMPLATE")

	cmd.Flags().StringVarP(&b.encoding, "encoding", "e", "", "Encoding for the input stream. If a file is provided will gather encoding type from file extension. If extension provided will override.")
	cmd.MarkFlagFilename("encoding", "yaml", "yml", "json", "jsonnet")
}

func (b *cmdTemplateBuilder) exportTemplate(w io.Writer, templateSVC pkger.SVC, outPath string, opts ...pkger.ExportOptFn) error {
	template, err := templateSVC.Export(context.Background(), opts...)
	if err != nil {
		return err
	}

	return b.writeTemplate(w, outPath, template)
}

func (b *cmdTemplateBuilder) writeTemplate(w io.Writer, outPath string, template *pkger.Template) error {
	buf, err := createTemplateBuf(template, outPath)
	if err != nil {
		return err
	}

	if outPath == "" {
		_, err := io.Copy(w, buf)
		return err
	}

	return ioutil.WriteFile(outPath, buf.Bytes(), os.ModePerm)
}

func (b *cmdTemplateBuilder) readRawTemplatesFromFiles(filePaths []string, recurse bool) ([]*pkger.Template, error) {
	mFiles := make(map[string]struct{})
	for _, f := range filePaths {
		files, err := readFilesFromPath(f, recurse)
		if err != nil {
			return nil, err
		}
		for _, ff := range files {
			mFiles[ff] = struct{}{}
		}
	}

	var rawTemplates []*pkger.Template
	for f := range mFiles {
		template, err := pkger.Parse(b.convertFileEncoding(f), pkger.FromFile(f), pkger.ValidSkipParseError())
		if err != nil {
			return nil, err
		}
		rawTemplates = append(rawTemplates, template)
	}

	return rawTemplates, nil
}

func (b *cmdTemplateBuilder) readRawTemplatesFromURLs(urls []string) ([]*pkger.Template, error) {
	mURLs := make(map[string]struct{})
	for _, f := range urls {
		mURLs[f] = struct{}{}
	}

	var rawTemplates []*pkger.Template
	for u := range mURLs {
		template, err := pkger.Parse(b.convertURLEncoding(u), pkger.FromHTTPRequest(u), pkger.ValidSkipParseError())
		if err != nil {
			return nil, err
		}
		rawTemplates = append(rawTemplates, template)
	}
	return rawTemplates, nil
}

func (b *cmdTemplateBuilder) readTemplate() (*pkger.Template, bool, error) {
	var remotes, files []string
	for _, rawURL := range append(b.files, b.urls...) {
		u, err := url.Parse(rawURL)
		if err != nil {
			return nil, false, ierror.Wrap(err, fmt.Sprintf("failed to parse url[%s]", rawURL))
		}
		if strings.HasPrefix(u.Scheme, "http") {
			remotes = append(remotes, u.String())
		} else {
			files = append(files, u.String())
		}
	}

	templates, err := b.readRawTemplatesFromFiles(files, b.recurse)
	if err != nil {
		return nil, false, err
	}

	urlTemplates, err := b.readRawTemplatesFromURLs(remotes)
	if err != nil {
		return nil, false, err
	}
	templates = append(templates, urlTemplates...)

	// the pkger.ValidSkipParseError option allows our server to be the one to validate the
	// the template is accurate. If a user has an older version of the CLI and cloud gets updated
	// with new validation rules,they'll get immediate access to that change without having to
	// rol their CLI build.

	if _, err := inStdIn(b.in); err != nil {
		template, err := pkger.Combine(templates, pkger.ValidSkipParseError())
		return template, false, err
	}

	stdinTemplate, err := pkger.Parse(b.convertEncoding(), pkger.FromReader(b.in), pkger.ValidSkipParseError())
	if err != nil {
		return nil, true, err
	}

	template, err := pkger.Combine(append(templates, stdinTemplate), pkger.ValidSkipParseError())
	return template, true, err
}

func (b *cmdTemplateBuilder) readLines(r io.Reader) ([]string, error) {
	bb, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	var stdinInput []string
	for _, bs := range bytes.Split(bb, []byte("\n")) {
		trimmed := bytes.TrimSpace(bs)
		if len(trimmed) == 0 {
			continue
		}
		stdinInput = append(stdinInput, string(trimmed))
	}
	return stdinInput, nil
}

func (b *cmdTemplateBuilder) getInput(msg, defaultVal string) string {
	ui := &input.UI{
		Writer: b.w,
		Reader: b.in,
	}
	return internal2.GetInput(ui, msg, defaultVal)
}

func (b *cmdTemplateBuilder) convertURLEncoding(url string) pkger.Encoding {
	urlBase := path.Ext(url)
	switch {
	case strings.HasPrefix(urlBase, ".jsonnet"):
		return pkger.EncodingJsonnet
	case strings.HasPrefix(urlBase, ".json"):
		return pkger.EncodingJSON
	case strings.HasPrefix(urlBase, ".yml") || strings.HasPrefix(urlBase, ".yaml"):
		return pkger.EncodingYAML
	}
	return b.convertEncoding()
}

func (b *cmdTemplateBuilder) convertFileEncoding(file string) pkger.Encoding {
	ext := filepath.Ext(file)
	switch {
	case strings.HasPrefix(ext, ".jsonnet"):
		return pkger.EncodingJsonnet
	case strings.HasPrefix(ext, ".json"):
		return pkger.EncodingJSON
	case strings.HasPrefix(ext, ".yml") || strings.HasPrefix(ext, ".yaml"):
		return pkger.EncodingYAML
	}

	return b.convertEncoding()
}

func (b *cmdTemplateBuilder) convertEncoding() pkger.Encoding {
	switch {
	case b.encoding == "json":
		return pkger.EncodingJSON
	case b.encoding == "yml" || b.encoding == "yaml":
		return pkger.EncodingYAML
	case b.encoding == "jsonnet":
		return pkger.EncodingJsonnet
	default:
		return pkger.EncodingSource
	}
}

func newResourcesToClone(kind pkger.Kind, idStrs, names []string) (pkger.ExportOptFn, error) {
	ids, err := toInfluxIDs(idStrs)
	if err != nil {
		return nil, err
	}

	var resources []pkger.ResourceToClone
	for _, id := range ids {
		resources = append(resources, pkger.ResourceToClone{
			Kind: kind,
			ID:   id,
		})
	}
	for _, name := range names {
		if len(name) == 0 {
			continue
		}
		resources = append(resources, pkger.ResourceToClone{
			Kind: kind,
			Name: name,
		})
	}
	return pkger.ExportWithExistingResources(resources...), nil
}

func toInfluxIDs(args []string) ([]platform.ID, error) {
	var (
		ids  []platform.ID
		errs []string
	)
	for _, arg := range args {
		normedArg := strings.TrimSpace(strings.ToLower(arg))
		if len(normedArg) == 0 {
			continue
		}

		id, err := platform.IDFromString(normedArg)
		if err != nil {
			errs = append(errs, "arg must provide a valid 16 length ID; got: "+arg)
			continue
		}
		ids = append(ids, *id)
	}
	if len(errs) > 0 {
		return nil, errors.New(strings.Join(errs, "\n\t"))
	}

	return ids, nil
}

func createTemplateBuf(template *pkger.Template, outPath string) (*bytes.Buffer, error) {
	var encoding pkger.Encoding
	switch ext := filepath.Ext(outPath); ext {
	case ".json":
		encoding = pkger.EncodingJSON
	default:
		encoding = pkger.EncodingYAML
	}

	b, err := template.Encode(encoding)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(b), nil
}

func newPkgerSVC() (pkger.SVC, influxdb.OrganizationService, error) {
	httpClient, err := newHTTPClient()
	if err != nil {
		return nil, nil, err
	}

	orgSvc := &tenant.OrgClientService{
		Client: httpClient,
	}

	return &pkger.HTTPRemoteService{Client: httpClient}, orgSvc, nil
}

func (b *cmdTemplateBuilder) printTemplateDiff(diff pkger.Diff) error {
	if b.quiet {
		return nil
	}

	if b.json {
		return b.writeJSON(diff)
	}

	diffPrinterGen := func(title string, headers []string) *diffPrinter {
		commonHeaders := []string{"Metadata Name", "ID", "Resource Name"}

		printer := newDiffPrinter(b.w, !b.disableColor, !b.disableTableBorders)
		printer.
			Title(title).
			SetHeaders(append(commonHeaders, headers...)...)
		return printer
	}

	if labels := diff.Labels; len(labels) > 0 {
		printer := diffPrinterGen("Labels", []string{"Color", "Description"})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffLabelValues) []string {
			return []string{metaName, id.String(), v.Name, v.Color, v.Description}
		}

		for _, l := range labels {
			var oldRow []string
			if l.Old != nil {
				oldRow = appendValues(l.ID, l.MetaName, *l.Old)
			}

			newRow := appendValues(l.ID, l.MetaName, l.New)
			switch {
			case pkger.IsNew(l.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(l.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if bkts := diff.Buckets; len(bkts) > 0 {
		printer := diffPrinterGen("Buckets", []string{"Retention Period", "Description"})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffBucketValues) []string {
			return []string{metaName, id.String(), v.Name, v.RetentionRules.RP().String(), v.Description}
		}

		for _, b := range bkts {
			var oldRow []string
			if b.Old != nil {
				oldRow = appendValues(b.ID, b.MetaName, *b.Old)
			}

			newRow := appendValues(b.ID, b.MetaName, b.New)
			switch {
			case pkger.IsNew(b.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(b.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if checks := diff.Checks; len(checks) > 0 {
		printer := diffPrinterGen("Checks", []string{"Description"})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffCheckValues) []string {
			out := []string{metaName, id.String()}
			if v.Check == nil {
				return append(out, "", "")
			}
			return append(out, v.Check.GetName(), v.Check.GetDescription())
		}

		for _, c := range checks {
			var oldRow []string
			if c.Old != nil {
				oldRow = appendValues(c.ID, c.MetaName, *c.Old)
			}

			newRow := appendValues(c.ID, c.MetaName, c.New)
			switch {
			case pkger.IsNew(c.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(c.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if dashes := diff.Dashboards; len(dashes) > 0 {
		printer := diffPrinterGen("Dashboards", []string{"Description", "Num Charts"})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffDashboardValues) []string {
			return []string{metaName, id.String(), v.Name, v.Desc, strconv.Itoa(len(v.Charts))}
		}

		for _, d := range dashes {
			var oldRow []string
			if d.Old != nil {
				oldRow = appendValues(d.ID, d.MetaName, *d.Old)
			}

			newRow := appendValues(d.ID, d.MetaName, d.New)
			switch {
			case pkger.IsNew(d.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(d.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if endpoints := diff.NotificationEndpoints; len(endpoints) > 0 {
		printer := diffPrinterGen("Notification Endpoints", nil)

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffNotificationEndpointValues) []string {
			out := []string{metaName, id.String()}
			if v.NotificationEndpoint == nil {
				return append(out, "")
			}
			return append(out, v.NotificationEndpoint.GetName())
		}

		for _, e := range endpoints {
			var oldRow []string
			if e.Old != nil {
				oldRow = appendValues(e.ID, e.MetaName, *e.Old)
			}

			newRow := appendValues(e.ID, e.MetaName, e.New)
			switch {
			case pkger.IsNew(e.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(e.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if rules := diff.NotificationRules; len(rules) > 0 {
		printer := diffPrinterGen("Notification Rules", []string{
			"Description",
			"Every",
			"Offset",
			"Endpoint Name",
			"Endpoint ID",
			"Endpoint Type",
		})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffNotificationRuleValues) []string {
			return []string{
				metaName,
				id.String(),
				v.Name,
				v.Every,
				v.Offset,
				v.EndpointName,
				v.EndpointID.String(),
				v.EndpointType,
			}
		}

		for _, e := range rules {
			var oldRow []string
			if e.Old != nil {
				oldRow = appendValues(e.ID, e.MetaName, *e.Old)
			}

			newRow := appendValues(e.ID, e.MetaName, e.New)
			switch {
			case pkger.IsNew(e.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(e.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if teles := diff.Telegrafs; len(teles) > 0 {
		printer := diffPrinterGen("Telegraf Configurations", []string{"Description"})
		appendValues := func(id pkger.SafeID, metaName string, v influxdb.TelegrafConfig) []string {
			return []string{metaName, id.String(), v.Name, v.Description}
		}

		for _, e := range teles {
			var oldRow []string
			if e.Old != nil {
				oldRow = appendValues(e.ID, e.MetaName, *e.Old)
			}

			newRow := appendValues(e.ID, e.MetaName, e.New)
			switch {
			case pkger.IsNew(e.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(e.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if tasks := diff.Tasks; len(tasks) > 0 {
		printer := diffPrinterGen("Tasks", []string{"Description", "Cycle"})
		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffTaskValues) []string {
			timing := v.Cron
			if v.Cron == "" {
				if v.Offset == "" {
					v.Offset = time.Duration(0).String()
				}
				timing = fmt.Sprintf("every: %s offset: %s", v.Every, v.Offset)
			}
			return []string{metaName, id.String(), v.Name, v.Description, timing}
		}

		for _, e := range tasks {
			var oldRow []string
			if e.Old != nil {
				oldRow = appendValues(e.ID, e.MetaName, *e.Old)
			}

			newRow := appendValues(e.ID, e.MetaName, e.New)
			switch {
			case pkger.IsNew(e.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(e.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if vars := diff.Variables; len(vars) > 0 {
		printer := diffPrinterGen("Variables", []string{"Description", "Arg Type", "Arg Values"})

		appendValues := func(id pkger.SafeID, metaName string, v pkger.DiffVariableValues) []string {
			var argType string
			if v.Args != nil {
				argType = v.Args.Type
			}
			return []string{metaName, id.String(), v.Name, v.Description, argType, printVarArgs(v.Args)}
		}

		for _, v := range vars {
			var oldRow []string
			if v.Old != nil {
				oldRow = appendValues(v.ID, v.MetaName, *v.Old)
			}

			newRow := appendValues(v.ID, v.MetaName, v.New)
			switch {
			case pkger.IsNew(v.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(v.StateStatus):
				printer.AppendDiff(oldRow, nil)
			default:
				printer.AppendDiff(oldRow, newRow)
			}
		}
		printer.Render()
	}

	if len(diff.LabelMappings) > 0 {
		printer := newDiffPrinter(b.w, !b.disableColor, !b.disableTableBorders)
		printer.
			Title("Label Associations").
			SetHeaders(
				"Resource Type",
				"Resource Meta Name", "Resource Name", "Resource ID",
				"Label Package Name", "Label Name", "Label ID",
			)

		for _, m := range diff.LabelMappings {
			newRow := []string{
				string(m.ResType),
				m.ResMetaName, m.ResName, m.ResID.String(),
				m.LabelMetaName, m.LabelName, m.LabelID.String(),
			}
			switch {
			case pkger.IsNew(m.StateStatus):
				printer.AppendDiff(nil, newRow)
			case pkger.IsRemoval(m.StateStatus):
				printer.AppendDiff(newRow, nil)
			default:
				printer.AppendDiff(newRow, newRow)
			}
		}
		printer.Render()
	}

	return nil
}

func (b *cmdTemplateBuilder) printTemplateSummary(stackID platform.ID, sum pkger.Summary) error {
	if b.quiet {
		return nil
	}

	if b.json {
		return b.writeJSON(struct {
			StackID string `json:"stackID"`
			Summary pkger.Summary
		}{
			StackID: stackID.String(),
			Summary: sum,
		})
	}

	commonHeaders := []string{"Package Name", "ID", "Resource Name"}

	tablePrintFn := b.tablePrinterGen()
	if labels := sum.Labels; len(labels) > 0 {
		headers := append(commonHeaders, "Description", "Color")
		tablePrintFn("LABELS", headers, len(labels), func(i int) []string {
			l := labels[i]
			return []string{
				l.MetaName,
				l.ID.String(),
				l.Name,
				l.Properties.Description,
				l.Properties.Color,
			}
		})
	}

	if buckets := sum.Buckets; len(buckets) > 0 {
		headers := append(commonHeaders, "Retention", "Description")
		tablePrintFn("BUCKETS", headers, len(buckets), func(i int) []string {
			bucket := buckets[i]
			return []string{
				bucket.MetaName,
				bucket.ID.String(),
				bucket.Name,
				formatDuration(bucket.RetentionPeriod),
				bucket.Description,
			}
		})
	}

	if checks := sum.Checks; len(checks) > 0 {
		headers := append(commonHeaders, "Description")
		tablePrintFn("CHECKS", headers, len(checks), func(i int) []string {
			c := checks[i].Check
			return []string{
				checks[i].MetaName,
				c.GetID().String(),
				c.GetName(),
				c.GetDescription(),
			}
		})
	}

	if dashes := sum.Dashboards; len(dashes) > 0 {
		headers := append(commonHeaders, "Description")
		tablePrintFn("DASHBOARDS", headers, len(dashes), func(i int) []string {
			d := dashes[i]
			return []string{d.MetaName, d.ID.String(), d.Name, d.Description}
		})
	}

	if endpoints := sum.NotificationEndpoints; len(endpoints) > 0 {
		headers := append(commonHeaders, "Description", "Status")
		tablePrintFn("NOTIFICATION ENDPOINTS", headers, len(endpoints), func(i int) []string {
			v := endpoints[i]
			return []string{
				v.MetaName,
				v.NotificationEndpoint.GetID().String(),
				v.NotificationEndpoint.GetName(),
				v.NotificationEndpoint.GetDescription(),
				string(v.NotificationEndpoint.GetStatus()),
			}
		})
	}

	if rules := sum.NotificationRules; len(rules) > 0 {
		headers := append(commonHeaders, "Description", "Every", "Offset", "Endpoint Name", "Endpoint ID", "Endpoint Type")
		tablePrintFn("NOTIFICATION RULES", headers, len(rules), func(i int) []string {
			v := rules[i]
			return []string{
				v.MetaName,
				v.ID.String(),
				v.Name,
				v.Description,
				v.Every,
				v.Offset,
				v.EndpointMetaName,
				v.EndpointID.String(),
				v.EndpointType,
			}
		})
	}

	if tasks := sum.Tasks; len(tasks) > 0 {
		headers := append(commonHeaders, "Description", "Cycle")
		tablePrintFn("TASKS", headers, len(tasks), func(i int) []string {
			t := tasks[i]
			timing := fmt.Sprintf("every: %s offset: %s", t.Every, t.Offset)
			if t.Cron != "" {
				timing = t.Cron
			}
			return []string{
				t.MetaName,
				t.ID.String(),
				t.Name,
				t.Description,
				timing,
			}
		})
	}

	if teles := sum.TelegrafConfigs; len(teles) > 0 {
		headers := append(commonHeaders, "Description")
		tablePrintFn("TELEGRAF CONFIGS", headers, len(teles), func(i int) []string {
			t := teles[i]
			return []string{
				t.MetaName,
				t.TelegrafConfig.ID.String(),
				t.TelegrafConfig.Name,
				t.TelegrafConfig.Description,
			}
		})
	}

	if vars := sum.Variables; len(vars) > 0 {
		headers := append(commonHeaders, "Description", "Arg Type", "Arg Values")
		tablePrintFn("VARIABLES", headers, len(vars), func(i int) []string {
			v := vars[i]
			args := v.Arguments
			return []string{
				v.MetaName,
				v.ID.String(),
				v.Name,
				v.Description,
				args.Type,
				printVarArgs(args),
			}
		})
	}

	if mappings := sum.LabelMappings; len(mappings) > 0 {
		headers := []string{"Resource Type", "Resource Name", "Resource ID", "Label Name", "Label ID"}
		tablePrintFn("LABEL ASSOCIATIONS", headers, len(mappings), func(i int) []string {
			m := mappings[i]
			return []string{
				string(m.ResourceType),
				m.ResourceName,
				m.ResourceID.String(),
				m.LabelName,
				m.LabelID.String(),
			}
		})
	}

	if secrets := sum.MissingSecrets; len(secrets) > 0 {
		headers := []string{"Secret Key"}
		tablePrintFn("MISSING SECRETS", headers, len(secrets), func(i int) []string {
			return []string{secrets[i]}
		})
	}

	if stackID != 0 {
		fmt.Fprintln(b.w, "Stack ID: "+stackID.String())
	}

	return nil
}

func (b *cmdTemplateBuilder) tablePrinterGen() func(table string, headers []string, count int, rowFn func(i int) []string) {
	return func(table string, headers []string, count int, rowFn func(i int) []string) {
		tablePrinter(b.w, table, headers, count, !b.disableColor, !b.disableTableBorders, rowFn)
	}
}

func writeStackRows(tabW *internal.TabWriter, stacks ...pkger.Stack) {
	tabW.WriteHeaders("ID", "OrgID", "Active", "Name", "Description", "Num Resources", "Sources", "URLs", "Created At", "Updated At")
	for _, stack := range stacks {
		latest := stack.LatestEvent()
		tabW.Write(map[string]interface{}{
			"ID":            stack.ID,
			"OrgID":         stack.OrgID,
			"Active":        latest.EventType != pkger.StackEventUninstalled,
			"Name":          latest.Name,
			"Description":   latest.Description,
			"Num Resources": len(latest.Resources),
			"Sources":       latest.Sources,
			"URLs":          latest.TemplateURLs,
			"Created At":    stack.CreatedAt,
			"Updated At":    latest.UpdatedAt,
		})
	}
}

type diffPrinter struct {
	w      io.Writer
	writer *tablewriter.Table

	colorAdd     tablewriter.Colors
	colorFooter  tablewriter.Colors
	colorHeaders tablewriter.Colors
	colorRemove  tablewriter.Colors
	title        string

	appendCalls int
	headerLen   int
}

func newDiffPrinter(w io.Writer, hasColor, hasBorder bool) *diffPrinter {
	wr := tablewriter.NewWriter(w)
	wr.SetBorder(hasBorder)
	wr.SetRowLine(hasBorder)

	var (
		colorAdd    = tablewriter.Colors{}
		colorFooter = tablewriter.Colors{}
		colorHeader = tablewriter.Colors{}
		colorRemove = tablewriter.Colors{}
	)
	if hasColor {
		colorAdd = tablewriter.Colors{tablewriter.FgHiGreenColor, tablewriter.Bold}
		colorFooter = tablewriter.Color(tablewriter.FgHiBlueColor, tablewriter.Bold)
		colorHeader = tablewriter.Colors{tablewriter.FgCyanColor, tablewriter.Bold}
		colorRemove = tablewriter.Colors{tablewriter.FgRedColor, tablewriter.Bold}
	}
	return &diffPrinter{
		w:            w,
		writer:       wr,
		colorAdd:     colorAdd,
		colorFooter:  colorFooter,
		colorHeaders: colorHeader,
		colorRemove:  colorRemove,
	}
}

func (d *diffPrinter) Render() {
	if d.appendCalls == 0 {
		return
	}

	// set the title and the add/remove legend
	title := color.New(color.FgYellow, color.Bold).Sprint(strings.ToUpper(d.title))
	add := color.New(color.FgHiGreen, color.Bold).Sprint("+add")
	remove := color.New(color.FgRed, color.Bold).Sprint("-remove")
	fmt.Fprintf(d.w, "%s    %s | %s | unchanged\n", title, add, remove)

	d.setFooter()
	d.writer.Render()
	fmt.Fprintln(d.w)
}

func (d *diffPrinter) Title(title string) *diffPrinter {
	d.title = title
	return d
}

func (d *diffPrinter) SetHeaders(headers ...string) *diffPrinter {
	headers = d.prepend(headers, "+/-")
	d.headerLen = len(headers)

	d.writer.SetHeader(headers)

	headerColors := make([]tablewriter.Colors, d.headerLen)
	for i := range headerColors {
		headerColors[i] = d.colorHeaders
	}
	d.writer.SetHeaderColor(headerColors...)

	return d
}

func (d *diffPrinter) setFooter() *diffPrinter {
	footers := make([]string, d.headerLen)
	if d.headerLen > 1 {
		footers[len(footers)-2] = "TOTAL"
		footers[len(footers)-1] = strconv.Itoa(d.appendCalls)
	} else {
		footers[0] = "TOTAL: " + strconv.Itoa(d.appendCalls)
	}

	d.writer.SetFooter(footers)
	colors := make([]tablewriter.Colors, d.headerLen)
	if d.headerLen > 1 {
		colors[len(colors)-2] = d.colorFooter
		colors[len(colors)-1] = d.colorFooter
	} else {
		colors[0] = d.colorFooter
	}
	d.writer.SetFooterColor(colors...)

	return d
}

func (d *diffPrinter) Append(slc []string) {
	d.writer.Append(d.prepend(slc, ""))
}

func (d *diffPrinter) AppendDiff(remove, add []string) {
	defer func() { d.appendCalls++ }()

	if d.appendCalls > 0 {
		d.appendBufferLine()
	}

	lenAdd, lenRemove := len(add), len(remove)
	preppedAdd, preppedRemove := d.prepend(add, "+"), d.prepend(remove, "-")
	if lenRemove > 0 && lenAdd == 0 {
		d.writer.Rich(preppedRemove, d.redRow(len(preppedRemove)))
		return
	}
	if lenAdd > 0 && lenRemove == 0 {
		d.writer.Rich(preppedAdd, d.greenRow(len(preppedAdd)))
		return
	}

	var (
		addColors    = make([]tablewriter.Colors, len(preppedAdd))
		removeColors = make([]tablewriter.Colors, len(preppedRemove))
		hasDiff      bool
	)
	for i := 0; i < lenRemove; i++ {
		if add[i] != remove[i] {
			hasDiff = true
			// offset to skip prepended +/- column
			addColors[i+1], removeColors[i+1] = d.colorAdd, d.colorRemove
		}
	}

	if !hasDiff {
		d.writer.Append(d.prepend(add, ""))
		return
	}

	addColors[0], removeColors[0] = d.colorAdd, d.colorRemove
	d.writer.Rich(d.prepend(remove, "-"), removeColors)
	d.writer.Rich(d.prepend(add, "+"), addColors)
}

func (d *diffPrinter) appendBufferLine() {
	d.writer.Append([]string{})
}

func (d *diffPrinter) redRow(i int) []tablewriter.Colors {
	return colorRow(d.colorRemove, i)
}

func (d *diffPrinter) greenRow(i int) []tablewriter.Colors {
	return colorRow(d.colorAdd, i)
}

func (d *diffPrinter) prepend(slc []string, val string) []string {
	return append([]string{val}, slc...)
}

func colorRow(color tablewriter.Colors, i int) []tablewriter.Colors {
	colors := make([]tablewriter.Colors, i)
	for i := range colors {
		colors[i] = color
	}
	return colors
}

func tablePrinter(wr io.Writer, table string, headers []string, count int, hasColor, hasTableBorders bool, rowFn func(i int) []string) {
	color.New(color.FgYellow, color.Bold).Fprintln(wr, strings.ToUpper(table))

	w := tablewriter.NewWriter(wr)
	w.SetBorder(hasTableBorders)
	w.SetRowLine(hasTableBorders)

	var alignments []int
	for range headers {
		alignments = append(alignments, tablewriter.ALIGN_CENTER)
	}

	descrCol := find("description", headers)
	if descrCol != -1 {
		w.SetColMinWidth(descrCol, 30)
		alignments[descrCol] = tablewriter.ALIGN_LEFT
	}

	w.SetHeader(headers)
	w.SetColumnAlignment(alignments)

	for i := range make([]struct{}, count) {
		w.Append(rowFn(i))
	}

	footers := make([]string, len(headers))
	if len(headers) > 1 {
		footers[len(footers)-2] = "TOTAL"
		footers[len(footers)-1] = strconv.Itoa(count)
	} else {
		footers[0] = "TOTAL: " + strconv.Itoa(count)
	}
	w.SetFooter(footers)
	if hasColor {
		headerColor := tablewriter.Color(tablewriter.FgHiCyanColor, tablewriter.Bold)
		footerColor := tablewriter.Color(tablewriter.FgHiBlueColor, tablewriter.Bold)

		var colors []tablewriter.Colors
		for i := 0; i < len(headers); i++ {
			colors = append(colors, headerColor)
		}
		w.SetHeaderColor(colors...)
		if len(headers) > 1 {
			colors[len(colors)-2] = footerColor
			colors[len(colors)-1] = footerColor
		} else {
			colors[0] = footerColor
		}
		w.SetFooterColor(colors...)
	}

	w.Render()
	fmt.Fprintln(wr)
}

func printVarArgs(a *influxdb.VariableArguments) string {
	if a == nil {
		return "<nil>"
	}
	if a.Type == "map" {
		b, err := json.Marshal(a.Values)
		if err != nil {
			return "{}"
		}
		return string(b)
	}
	if a.Type == "constant" {
		vals, ok := a.Values.(influxdb.VariableConstantValues)
		if !ok {
			return "[]"
		}
		var out []string
		for _, s := range vals {
			out = append(out, fmt.Sprintf("%q", s))
		}
		return fmt.Sprintf("[%s]", strings.Join(out, " "))
	}
	if a.Type == "query" {
		qVal, ok := a.Values.(influxdb.VariableQueryValues)
		if !ok {
			return ""
		}
		return fmt.Sprintf("language=%q query=%q", qVal.Language, qVal.Query)
	}
	return "unknown variable argument"
}

func formatDuration(d time.Duration) string {
	if d == 0 {
		return "inf"
	}
	return d.String()
}

func readFilesFromPath(filePath string, recurse bool) ([]string, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return []string{filePath}, nil
	}

	dirFiles, err := ioutil.ReadDir(filePath)
	if err != nil {
		return nil, err
	}

	mFiles := make(map[string]struct{})
	assign := func(ss ...string) {
		for _, s := range ss {
			mFiles[s] = struct{}{}
		}
	}
	for _, f := range dirFiles {
		fileP := filepath.Join(filePath, f.Name())
		if f.IsDir() {
			if recurse {
				rFiles, err := readFilesFromPath(fileP, recurse)
				if err != nil {
					return nil, err
				}
				assign(rFiles...)
			}
			continue
		}
		assign(fileP)
	}

	var files []string
	for f := range mFiles {
		files = append(files, f)
	}
	return files, nil
}

func mapKeys(provided, kvPairs []string) map[string]string {
	out := make(map[string]string)
	for _, k := range provided {
		out[k] = ""
	}

	for _, pair := range kvPairs {
		pieces := strings.SplitN(pair, "=", 2)
		if len(pieces) < 2 {
			continue
		}

		k, v := pieces[0], pieces[1]
		if _, ok := out[k]; !ok {
			continue
		}
		out[k] = v
	}

	return out
}

func toMapInterface(m map[string]string) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range m {
		out[k] = v
	}
	return out
}

func missingValKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		if v != "" {
			continue
		}
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i] < out[j]
	})
	return out
}

func templateKindFold(resType string) pkger.Kind {
	resKind := pkger.KindUnknown
	for _, k := range pkger.Kinds() {
		if strings.EqualFold(string(k), resType) {
			resKind = k
			break
		}
	}
	return resKind
}

func find(needle string, haystack []string) int {
	for i, h := range haystack {
		if strings.ToLower(h) == needle {
			return i
		}
	}
	return -1
}

func inStdIn(in io.Reader) (*os.File, error) {
	stdin, _ := in.(*os.File)
	if stdin != os.Stdin {
		return nil, errors.New("input not stdIn")
	}

	info, err := stdin.Stat()
	if err != nil {
		return nil, err
	}
	if (info.Mode() & os.ModeCharDevice) == os.ModeCharDevice {
		return nil, errors.New("input not stdIn")
	}
	return stdin, nil
}
