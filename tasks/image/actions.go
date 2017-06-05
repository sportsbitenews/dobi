package image

import (
	"fmt"

	"github.com/dnephin/dobi/config"
	"github.com/dnephin/dobi/tasks/context"
	"github.com/dnephin/dobi/tasks/task"
	"github.com/dnephin/dobi/tasks/types"
)

// GetTaskConfig returns a new TaskConfig for the action
func GetTaskConfig(name, action string, conf *config.ImageConfig) (types.TaskConfig, error) {
	var taskName task.Name

	if action == "" {
		action = defaultAction(conf)
		taskName = task.NewDefaultName(name, action)
	} else {
		taskName = task.NewName(name, action)
	}
	imageAction, err := getAction(action, name)
	if err != nil {
		return nil, err
	}
	return types.NewTaskConfig(
		taskName,
		conf,
		deps(conf, imageAction.dependencies),
		NewTask(imageAction.run),
	), nil
}

type runFunc func(*context.ExecuteContext, *Task, bool) (bool, error)

type action struct {
	name         string
	run          runFunc
	dependencies []string
}

func newAction(name string, run runFunc, deps []string) (action, error) {
	return action{name: name, run: run, dependencies: deps}, nil
}

func getAction(name string, task string) (action, error) {
	switch name {
	case "build":
		return newAction("build", RunBuild, nil)
	case "pull":
		return newAction("pull", RunPull, nil)
	case "push":
		return newAction("push", RunPush, imageDeps(task, "tag"))
	case "tag":
		return newAction("tag", RunTag, imageDeps(task, "build"))
	case "remove", "rm":
		return newAction("remove", RunRemove, nil)
	default:
		return action{}, fmt.Errorf("invalid image action %q for task %q", name, task)
	}
}

func defaultAction(conf *config.ImageConfig) string {
	if conf.Dockerfile != "" || conf.Steps != "" {
		return "build"
	}
	return "pull"
}

func imageDeps(name string, actions ...string) []string {
	deps := []string{}
	for _, action := range actions {
		deps = append(deps, task.NewName(name, action).Name())
	}
	return deps
}

func deps(conf config.Resource, deps []string) func() []string {
	return func() []string {
		return append(deps, conf.Dependencies()...)
	}
}

// NewTask creates a new Task object
func NewTask(runFunc runFunc) func(task.Name, config.Resource) types.Task {
	return func(name task.Name, conf config.Resource) types.Task {
		return &Task{name: name, config: conf.(*config.ImageConfig), runFunc: runFunc}
	}
}
