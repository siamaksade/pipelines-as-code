package repository

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cmd/completion"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/flags"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	header            = "NAME\tAGE\tOWNER/REPOSITORY\tSHA\tEVENT-TYPE"
	body              = "%s\t%s\t%s\t%s\t%s"
	allNamespacesFlag = "all-namespaces"
	namespaceFlag     = "namespace"
	noColorFlag       = "no-color"
)

func ListCommand(p cli.Params, ioStreams *ui.IOStreams) *cobra.Command {
	var noheaders, allNamespaces bool
	var selectors string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := flags.NewCliOptions(cmd)
			if err != nil {
				return err
			}

			opts.AllNameSpaces, err = cmd.Flags().GetBool(allNamespacesFlag)
			if err != nil {
				return err
			}
			opts.Namespace, err = cmd.Flags().GetString(namespaceFlag)
			if err != nil {
				return err
			}
			ioStreams.SetColorEnabled(!opts.NoColoring)

			cs, err := p.Clients()
			if err != nil {
				return err
			}
			ctx := context.Background()
			cw := clockwork.NewRealClock()
			return list(ctx, cs, opts, ioStreams, cw, p.GetNamespace(), selectors, noheaders)
		},
	}

	cmd.PersistentFlags().BoolP(noColorFlag, "C", !ioStreams.ColorEnabled(), "disable coloring")
	cmd.PersistentFlags().BoolVarP(&allNamespaces, allNamespacesFlag, "A", false, "If present, "+
		"list the repository across all namespaces. Namespace in current context is ignored even if specified with"+
		" --namespace.")

	cmd.Flags().StringP(
		namespaceFlag, "n", "", "If present, the namespace scope for this CLI request")

	_ = cmd.RegisterFlagCompletionFunc(namespaceFlag,
		func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return completion.BaseCompletion(namespaceFlag, args)
		},
	)

	cmd.Flags().BoolVar(
		&noheaders, "no-headers", false, "don't print headers.")

	cmd.Flags().StringVarP(&selectors, "selectors", "l",
		"", "Selector (label query) to filter on, "+
			"supports '=', "+
			"'==',"+
			" and '!='.(e.g. -l key1=value1,key2=value2)")
	return cmd
}

func list(ctx context.Context, cs *cli.Clients, opts *flags.CliOpts, ioStreams *ui.IOStreams,
	cw clockwork.Clock,
	currentNamespace, selectors string,
	noheaders bool) error {
	if opts.Namespace != "" {
		currentNamespace = opts.Namespace
	}
	if opts.AllNameSpaces {
		currentNamespace = ""
	}

	lopt := metav1.ListOptions{LabelSelector: selectors}

	repositories, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(currentNamespace).List(
		ctx, lopt)
	if err != nil {
		return err
	}

	w := tabwriter.NewWriter(ioStreams.Out, 0, 5, 3, ' ', tabwriter.TabIndent)

	if !noheaders {
		_, _ = fmt.Fprint(w, header)
		if opts.AllNameSpaces {
			fmt.Fprint(w, "\tNAMESPACE")
		}
		fmt.Fprintln(w, "\tSTATUS")
	}
	for _, repository := range repositories.Items {
		repoOwner, err := ui.GetRepoOwnerFromGHURL(repository.Spec.URL)
		if err != nil {
			return err
		}

		fmt.Fprintf(w, body,
			repository.GetName(),
			ui.ShowLastAge(repository, cw),
			repoOwner,
			ui.ShowLastSHA(repository),
			repository.Spec.EventType)

		if opts.AllNameSpaces {
			fmt.Fprintf(w, "\t%s", repository.GetNamespace())
		}

		fmt.Fprintf(w, "\t%s", ui.ShowStatus(repository, ioStreams.ColorScheme()))
		fmt.Fprint(w, "\n")
	}

	w.Flush()
	return nil
}
