package config

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode"
	apipac "github.com/openshift-pipelines/pipelines-as-code/pkg/apis/pipelinesascode/v1alpha1"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/cli"
	"github.com/openshift-pipelines/pipelines-as-code/pkg/webvcs"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
)

const (
	onEventAnnotation        = "on-event"
	onTargetBranchAnnotation = "on-target-branch"
	onTargetNamespace        = "target-namespace"
	reValidateTag            = `^\[(.*)\]$`
	maxKeepRuns              = "max-keep-runs"
)

// TODO: move to another file since it's common to all annotations_* files
func getAnnotationValues(annotation string) ([]string, error) {
	re := regexp.MustCompile(reValidateTag)
	match := re.Match([]byte(annotation))
	if !match {
		return nil, errors.New("annotations in pipeline are in wrong format")
	}

	// Split all tasks by comma and make sure to trim spaces in there
	splitted := strings.Split(re.FindStringSubmatch(annotation)[1], ",")
	for i := range splitted {
		splitted[i] = strings.TrimSpace(splitted[i])
	}

	if splitted[0] == "" {
		return nil, errors.New("annotations in pipeline are empty")
	}

	return splitted, nil
}

func MatchPipelinerunByAnnotation(ctx context.Context, pruns []*v1beta1.PipelineRun, cs *cli.Clients,
	runinfo *webvcs.RunInfo) (*v1beta1.PipelineRun, *apipac.Repository, map[string]string, error) {
	configurations := map[string]map[string]string{}
	repo := &apipac.Repository{}
	for _, prun := range pruns {
		configurations[prun.GetGenerateName()] = map[string]string{}
		if prun.GetObjectMeta().GetAnnotations() == nil {
			cs.Log.Warnf("PipelineRun %s does not have any annotations", prun.GetName())
			continue
		}

		if maxPrNumber, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+maxKeepRuns]; ok {
			configurations[prun.GetGenerateName()]["max-keep-runs"] = maxPrNumber
		}

		if targetNS, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onTargetNamespace]; ok {
			configurations[prun.GetGenerateName()]["target-namespace"] = targetNS
			repo, _ = GetRepoByCR(ctx, cs, targetNS, runinfo)
			if repo == nil {
				cs.Log.Warnf("could not find Repository CRD in %s while pipelineRun %s targets it", targetNS, prun.GetGenerateName())
				continue
			}
		}

		if targetEvent, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onEventAnnotation]; ok {
			matched, err := matchOnAnnotation(targetEvent, runinfo.EventType, false)
			configurations[prun.GetGenerateName()]["target-event"] = targetEvent
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if !matched {
				continue
			}
		}

		if targetBranch, ok := prun.GetObjectMeta().GetAnnotations()[pipelinesascode.
			GroupName+"/"+onTargetBranchAnnotation]; ok {
			matched, err := matchOnAnnotation(targetBranch, runinfo.BaseBranch, true)
			configurations[prun.GetGenerateName()]["target-branch"] = targetBranch
			if err != nil {
				return nil, nil, map[string]string{}, err
			}
			if !matched {
				continue
			}
		}

		return prun, repo, configurations[prun.GetGenerateName()], nil
	}

	cs.Log.Infof("cannot match between event and pipelineRuns: URL=%s baseBranch=%s, "+
		"eventType=%s", runinfo.URL,
		runinfo.BaseBranch,
		runinfo.EventType)

	cs.Log.Info("available configuration in pipelineRuns annotations")
	for prunname, maps := range configurations {
		cs.Log.Infof("pipelineRun: %s, baseBranch=%s, targetEvent=%s, targetNs=%s",
			prunname, maps["target-branch"], maps["target-event"], maps["target-namespace"])
	}

	// TODO: more descriptive error message
	return nil, nil, map[string]string{}, fmt.Errorf("cannot match pipeline from webhook to pipelineruns")
}

func matchOnAnnotation(annotations string, runinfoValue string, branchMatching bool) (bool, error) {
	targets, err := getAnnotationValues(annotations)
	if err != nil {
		return false, err
	}

	var gotit string
	for _, v := range targets {
		if v == runinfoValue {
			gotit = v
		}
		if branchMatching && branchMatch(v, runinfoValue) {
			gotit = v
		}
	}
	if gotit == "" {
		return false, nil
	}
	return true, nil
}
