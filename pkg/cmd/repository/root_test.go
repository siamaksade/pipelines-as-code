package repository

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/jonboulle/clockwork"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli/ui"
	tcli "github.com/openshift-pipelines/pipelines-as-code/pkg/test/cli"
	testclient "github.com/openshift-pipelines/pipelines-as-code/pkg/test/clients"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/test/params"
	"github.com/spf13/cobra"
	"gotest.tools/v3/assert"
	"gotest.tools/v3/golden"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	knativeapis "knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck/v1beta1"
	rtesting "knative.dev/pkg/reconciler/testing"
)

func TestRoot(t *testing.T) {
	s, err := tcli.ExecuteCommand(Root(&cli.PacParams{}), "help")
	assert.NilError(t, err)
	assert.Assert(t, strings.Contains(s, "repository, repo, repsitories"))
}

func TestCommands(t *testing.T) {
	tests := []struct {
		name    string
		command func(p cli.Params, ioStreams *ui.IOStreams) *cobra.Command
		want    *cobra.Command
	}{
		{
			name:    "List",
			command: ListCommand,
		},
		{
			name:    "Describe",
			command: DescribeCommand,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cw := clockwork.NewFakeClock()
			const (
				nsName   = "ns"
				repoName = "repo1"
			)
			statuses := []v1alpha1.RepositoryRunStatus{
				{
					Status: v1beta1.Status{
						Conditions: []knativeapis.Condition{
							{
								Reason: "Success",
							},
						},
					},
					PipelineRunName: "pipelinerun1",
					StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
					CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
					SHA:             github.String("SHA"),
					SHAURL:          github.String("https://anurl.com/repo/owner/commit/SHA"),
					Title:           github.String("A title"),
				},
			}
			repositories := []*v1alpha1.Repository{
				{
					ObjectMeta: metav1.ObjectMeta{
						Name:      repoName,
						Namespace: nsName,
					},
					Spec: v1alpha1.RepositorySpec{
						Namespace: nsName,
						URL:       "https://anurl.com/repo/owner",
						Branch:    "branch",
						EventType: "pull_request",
					},
					Status: statuses,
				},
			}
			tdata := testclient.Data{
				Namespaces: []*corev1.Namespace{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name: nsName,
						},
					},
				},
				Repositories: repositories,
			}
			ctx, _ := rtesting.SetupFakeContext(t)
			stdata, _ := testclient.SeedTestData(t, ctx, tdata)
			cs := &cli.Clients{
				PipelineAsCode: stdata.PipelineAsCode,
			}
			buf := new(bytes.Buffer)
			ioStream := &ui.IOStreams{Out: buf, ErrOut: buf}
			cmd := tt.command(params.FakeParams{
				Fakeclients: cs,
			}, ioStream)
			cmd.SetOut(buf)
			_, err := tcli.ExecuteCommand(cmd, "--no-color", "-n", nsName, repoName)
			assert.NilError(t, err)

			golden.Assert(t, buf.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
		})
	}
}

func TESSS(t *testing.T) {
	cw := clockwork.NewFakeClock()
	const (
		nsName   = "ns"
		repoName = "repo1"
	)
	statuses := []v1alpha1.RepositoryRunStatus{
		{
			Status: v1beta1.Status{
				Conditions: []knativeapis.Condition{
					{
						Reason: "Success",
					},
				},
			},
			PipelineRunName: "pipelinerun1",
			StartTime:       &metav1.Time{Time: cw.Now().Add(-16 * time.Minute)},
			CompletionTime:  &metav1.Time{Time: cw.Now().Add(-15 * time.Minute)},
			SHA:             github.String("SHA"),
			SHAURL:          github.String("https://anurl.com/repo/owner/commit/SHA"),
			Title:           github.String("A title"),
		},
	}
	repositories := []*v1alpha1.Repository{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      repoName,
				Namespace: nsName,
			},
			Spec: v1alpha1.RepositorySpec{
				Namespace: nsName,
				URL:       "https://anurl.com/repo/owner",
				Branch:    "branch",
				EventType: "pull_request",
			},
			Status: statuses,
		},
	}
	tdata := testclient.Data{
		Namespaces: []*corev1.Namespace{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: nsName,
				},
			},
		},
		Repositories: repositories,
	}
	ctx, _ := rtesting.SetupFakeContext(t)
	stdata, _ := testclient.SeedTestData(t, ctx, tdata)
	cs := &cli.Clients{
		PipelineAsCode: stdata.PipelineAsCode,
	}
	buf := new(bytes.Buffer)
	cmd := ListCommand(params.FakeParams{
		Fakeclients: cs,
	}, &ui.IOStreams{Out: buf, ErrOut: buf})
	cmd.SetOut(buf)
	_, err := tcli.ExecuteCommand(cmd, "-n", nsName, repoName)
	assert.NilError(t, err)

	golden.Assert(t, buf.String(), strings.ReplaceAll(fmt.Sprintf("%s.golden", t.Name()), "/", "-"))
}
