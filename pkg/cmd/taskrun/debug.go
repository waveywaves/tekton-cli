// Copyright © 2021 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package taskrun

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/tektoncd/cli/pkg/cli"
	"github.com/tektoncd/cli/pkg/formatted"
	"github.com/tektoncd/cli/pkg/options"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/remotecommand"
	"strings"
	trlist "github.com/tektoncd/cli/pkg/taskrun/list"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "github.com/tektoncd/cli/pkg/debug"
	tr "github.com/tektoncd/cli/pkg/taskrun"
	corev1 "k8s.io/api/core/v1"
)

type DebugOptions struct {
	Params          cli.Params
	TaskrunName     string
	DebugTaskrunName string
	Last bool
	Limit int
	AskOpts survey.AskOpt
	Stream *cli.Stream
}

func (opts *DebugOptions) ValidateOpts() error {
	if opts.Limit <= 0 {
		return fmt.Errorf("limit was %d but must be a positive number", opts.Limit)
	}
	return nil
}

func (opts *DebugOptions) Ask(resource string, options []string) error {
	var ans string
	var qs = []*survey.Question{
		{
			Name: resource,
			Prompt: &survey.Select{
				Message: fmt.Sprintf("Select %s:", resource),
				Options: options,
			},
		},
	}

	if err := survey.Ask(qs, &ans, opts.AskOpts); err != nil {
		return err
	}

	switch resource {
	//case ResourceNamePipeline:
	//	opts.PipelineName = ans
	//case ResourceNamePipelineRun:
	//	opts.PipelineRunName = strings.Fields(ans)[0]
	//case ResourceNameTask:
	//	opts.TaskName = ans
	case "taskrun":
		opts.TaskrunName = strings.Fields(ans)[0]
		opts.DebugTaskrunName = strings.Fields(ans)[0]+"-debug"
	}

	return nil
}

func debugCommand(p cli.Params) *cobra.Command {
	opts := &DebugOptions{Params: p}
	eg := `
Rerun and debug a TaskRun named 'foo' from the namespace 'bar':

    tkn taskrun debug foo -n bar
`
	c := &cobra.Command{
		Use:          "debug",
		Short:        "Debug TaskRuns in a namespace",
		Example:      eg,
		SilenceUsage: true,
		Annotations: map[string]string{
			"commandType": "main",
		},
		ValidArgsFunction: formatted.ParentCompletion,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 0 {
				opts.TaskrunName = args[0]
			}

			opts.Stream = &cli.Stream{
				In: cmd.InOrStdin(),
				Out: cmd.OutOrStdout(),
				Err: cmd.OutOrStderr(),
			}

			return runDebug(opts)
		},
	}

	c.Flags().BoolVarP(&opts.Last, "last", "L", false, "debug the last TaskRun")
	c.Flags().IntVarP(&opts.Limit, "limit", "", defaultLimit, "lists number of TaskRuns")

	return c
}

func runDebug(opts *DebugOptions) error {
	if opts.TaskrunName == "" {
		if err := opts.ValidateOpts(); err != nil {
			return err
		}
		if err := askRunNameForDebug(opts); err != nil {
			return err
		}
	}

	cs, _ := opts.Params.Clients()

	taskrun, err := tr.GetV1beta1(cs, opts.TaskrunName, metav1.GetOptions{}, "default")
	if err != nil {
		return err
	}

	podName := taskrun.Status.PodName
	command := []string{
		"sh",
	}

	execOptions := &corev1.PodExecOptions{
		Command: command,
		Stdin: true,
		Stdout: true,
		Stderr: true,
		TTY: true,
		Container: "step-breakpoint",
	}

	request := cs.Kube.CoreV1().RESTClient().Post().Resource("pods").Name(podName).Namespace("default").
		SubResource("exec").
		Param("command","sh").
		Param("container", "step-breakpoint").
		Param("stdin","true").
		Param("stdout","true").
		Param("tty","true")

	request.VersionedParams(execOptions, runtime.NewParameterCodec(runtime.NewScheme()))

	exec, err := remotecommand.NewSPDYExecutor(&cs.RESTConfig, "POST", request.URL())
	if err != nil {
		return err
	}

	fmt.Println(request.URL().String())

	err = exec.Stream(remotecommand.StreamOptions{
		Stdin: opts.Stream.In,
		Stdout: opts.Stream.Out,
		Stderr: opts.Stream.Err,
		Tty: true,
	})
	if err != nil {
		return fmt.Errorf("error in Stream: %v", err)
	}


	//d, err := debug.NewDebugger()
	//if err != nil {
	//	return err
	//}
	//
	//err = d.DebugMode()
	//if err != nil {
	//	return err
	//}

	return nil
}

func askRunNameForDebug(opts *DebugOptions) error {
	lOpts := metav1.ListOptions{}

	trs, err := trlist.GetAllTaskRuns(opts.Params, lOpts, opts.Limit)
	if err != nil {
		return err
	}

	if len(trs) == 0 {
		return fmt.Errorf("No TaskRuns found")
	}

	if len(trs) == 1 || opts.Last {
		opts.TaskrunName = strings.Fields(trs[0])[0]
		opts.DebugTaskrunName = opts.TaskrunName+"-debug"
		return nil
	}

	return opts.Ask(options.ResourceNameTaskRun, trs)
}
