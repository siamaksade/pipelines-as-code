//go:build e2e
// +build e2e

package test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	pacv1alpha1 "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	tgithub "github.com/openshift-pipelines/pipelines-as-code/test/pkg/github"
	trepo "github.com/openshift-pipelines/pipelines-as-code/test/pkg/repository"
	twait "github.com/openshift-pipelines/pipelines-as-code/test/pkg/wait"
	"github.com/tektoncd/pipeline/pkg/names"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPush(t *testing.T) {
	targetNS := names.SimpleNameGenerator.RestrictLengthWithRandomSuffix("pac-e2e-push")
	targetBranch := targetNS
	targetEvent := "push"

	ctx := context.Background()
	cs, opts, err := setup()
	assert.NilError(t, err)

	repoinfo, resp, err := cs.GithubClient.Client.Repositories.Get(ctx, opts.Owner, opts.Repo)
	assert.NilError(t, err)
	if resp != nil && resp.Response.StatusCode == http.StatusNotFound {
		t.Errorf("Repository %s not found in %s", opts.Owner, opts.Repo)
	}

	repository := &pacv1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name: targetNS,
		},
		Spec: pacv1alpha1.RepositorySpec{
			Namespace: targetNS,
			URL:       repoinfo.GetHTMLURL(),
			EventType: targetEvent,
			Branch:    targetBranch,
		},
	}

	err = trepo.CreateNSRepo(ctx, targetNS, cs, repository)
	assert.NilError(t, err)

	entries := map[string]string{
		".tekton/run.yaml": fmt.Sprintf(`---
apiVersion: tekton.dev/v1beta1
kind: PipelineRun
metadata:
  name: pipeline
  annotations:
    pipelinesascode.tekton.dev/target-namespace: "%s"
    pipelinesascode.tekton.dev/on-target-branch: "[%s]"
    pipelinesascode.tekton.dev/on-event: "[%s]"
spec:
  pipelineSpec:
    tasks:
      - name: task
        taskSpec:
          steps:
            - name: task
              image: gcr.io/google-containers/busybox
              command: ["/bin/echo", "HELLOMOTO"]
`, targetNS, targetBranch, targetEvent),
	}

	targetRefName := fmt.Sprintf("refs/heads/%s", targetBranch)
	sha, err := tgithub.PushFilesToRef(ctx, cs.GithubClient.Client, "TestPush - "+targetBranch, repoinfo.GetDefaultBranch(), targetRefName, opts.Owner, opts.Repo, entries)
	cs.Log.Infof("Commit %s has been created and pushed to %s", sha, targetRefName)
	assert.NilError(t, err)
	defer tearDown(ctx, t, cs, -1, targetRefName, targetNS, opts)

	cs.Log.Infof("Waiting for Repository to be updated")
	err = twait.UntilRepositoryUpdated(ctx, cs.PipelineAsCode, targetNS, targetNS, 0, defaultTimeout)
	assert.NilError(t, err)

	cs.Log.Infof("Check if we have the repository set as succeeded")
	repo, err := cs.PipelineAsCode.PipelinesascodeV1alpha1().Repositories(targetNS).Get(ctx, targetNS, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Assert(t, repo.Status[len(repo.Status)-1].Conditions[0].Status == corev1.ConditionTrue)
}
