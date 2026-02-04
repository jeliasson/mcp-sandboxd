package runs

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	kuberneteserrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

type KubernetesExecutor struct {
	restConfig *rest.Config
	client     *kubernetes.Clientset
	namespace  string
	container  string
}

func NewKubernetesExecutor(restConfig *rest.Config, client *kubernetes.Clientset, namespace, container string) *KubernetesExecutor {
	return &KubernetesExecutor{restConfig: restConfig, client: client, namespace: namespace, container: container}
}

func (e *KubernetesExecutor) Exec(ctx context.Context, sandboxID string, params ExecParams, stdout, stderr io.Writer) (exitCode int, err error) {
	execCmd := e.wrapCommand(params)

	req := e.client.CoreV1().RESTClient().
		Post().
		Resource("pods").
		Name(sandboxID).
		Namespace(e.namespace).
		SubResource("exec")

	req.VersionedParams(&corev1.PodExecOptions{
		Container: e.container,
		Command:   execCmd,
		Stdout:    true,
		Stderr:    true,
		TTY:       false,
	}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(e.restConfig, "POST", req.URL())
	if err != nil {
		return 0, err
	}

	err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: stdout, Stderr: stderr, Tty: false})
	if err != nil {
		if code, ok := exitCodeFromExecError(err); ok {
			return code, nil
		}
		return 0, err
	}
	return 0, nil
}

func (e *KubernetesExecutor) CopyArtifacts(ctx context.Context, sandboxID string) (io.ReadCloser, error) {
	r, w := io.Pipe()
	go func() {
		defer w.Close()
		_, err := e.Exec(ctx, sandboxID, ExecParams{Cmd: []string{"tar", "-C", "/artifacts", "-cf", "-", "."}, Cwd: "/", AsUser: "root"}, w, io.Discard)
		if err != nil {
			_ = w.CloseWithError(err)
		}
	}()
	return r, nil
}

func (e *KubernetesExecutor) wrapCommand(params ExecParams) []string {
	cwd := params.Cwd
	if strings.TrimSpace(cwd) == "" {
		cwd = "/workspace"
	}

	cmdString := strings.Join(shellQuoteAll(params.Cmd), " ")
	script := "cd " + shellQuote(cwd) + " && "

	if len(params.Env) > 0 {
		assign := make([]string, 0, len(params.Env))
		for k, v := range params.Env {
			assign = append(assign, shellQuote(k+"="+v))
		}
		script += "env " + strings.Join(assign, " ") + " " + cmdString
	} else {
		script += cmdString
	}

	if params.AsUser == "root" {
		return []string{"bash", "-lc", script}
	}
	return []string{"setpriv", "--reuid=1000", "--regid=1000", "--init-groups", "--", "bash", "-lc", script}
}

func shellQuoteAll(args []string) []string {
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, shellQuote(a))
	}
	return out
}

func shellQuote(s string) string {
	// Minimal safe quoting for bash -lc.
	// Wrap in single quotes and escape embedded single quotes.
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

var exitCodeRe = regexp.MustCompile(`(?i)exit code:?\s*(\d+)`)

func exitCodeFromExecError(err error) (int, bool) {
	if se, ok := err.(*kuberneteserrors.StatusError); ok {
		st := se.ErrStatus
		if st.Reason == "NonZeroExitCode" {
			if st.Details != nil {
				for _, c := range st.Details.Causes {
					if c.Type == metav1.CauseType("ExitCode") {
						var code int
						_, parseErr := fmt.Sscanf(c.Message, "%d", &code)
						if parseErr == nil {
							return code, true
						}
					}
				}
			}
		}
	}

	// Fallback: parse from error string (covers some apiserver implementations).
	if m := exitCodeRe.FindStringSubmatch(err.Error()); len(m) == 2 {
		var code int
		_, parseErr := fmt.Sscanf(m[1], "%d", &code)
		if parseErr == nil {
			return code, true
		}
	}
	return 0, false
}
