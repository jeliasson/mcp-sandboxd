package runs

import (
	"context"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/jeliasson/mcp-sandboxd/internal/client/docker"
)

type DockerExecutor struct {
	docker *docker.Client
}

func NewDockerExecutor(docker *docker.Client) *DockerExecutor {
	return &DockerExecutor{docker: docker}
}

func (e *DockerExecutor) Exec(ctx context.Context, sandboxID string, params ExecParams, stdout, stderr io.Writer) (exitCode int, err error) {
	user := "1000:1000"
	if params.AsUser == "root" {
		user = "0"
	}

	env := make([]string, 0, len(params.Env))
	for k, v := range params.Env {
		env = append(env, k+"="+v)
	}

	execCfg := container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		WorkingDir:   params.Cwd,
		Env:          env,
		User:         user,
		Cmd:          params.Cmd,
	}

	execResp, err := e.docker.Raw().ContainerExecCreate(ctx, sandboxID, execCfg)
	if err != nil {
		return 0, err
	}

	attach, err := e.docker.Raw().ContainerExecAttach(ctx, execResp.ID, container.ExecAttachOptions{})
	if err != nil {
		return 0, err
	}
	defer attach.Close()

	_, _ = stdcopy.StdCopy(stdout, stderr, attach.Reader)

	exitCode, running, err := waitDockerExec(ctx, e.docker, execResp.ID, 10*time.Minute)
	if err != nil {
		return exitCode, err
	}
	if running {
		return exitCode, context.DeadlineExceeded
	}
	return exitCode, nil
}

func waitDockerExec(ctx context.Context, dockerClient *docker.Client, execID string, maxWait time.Duration) (exitCode int, running bool, err error) {
	deadline := time.Now().Add(maxWait)
	for {
		insp, err := dockerClient.Raw().ContainerExecInspect(ctx, execID)
		if err != nil {
			return 0, false, err
		}
		if !insp.Running {
			return insp.ExitCode, false, nil
		}
		if time.Now().After(deadline) {
			return insp.ExitCode, true, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func (e *DockerExecutor) CopyArtifacts(ctx context.Context, sandboxID string) (io.ReadCloser, error) {
	rc, _, err := e.docker.Raw().CopyFromContainer(ctx, sandboxID, "/artifacts")
	if err != nil {
		return nil, err
	}
	return rc, nil
}
