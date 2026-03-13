package probes_test

import (
	"crypto/rand"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ninedraft/httpext"
	"github.com/ninedraft/httpext/probes"
)

const envCmd = "GO_WANT_HELPER_PROCESS"

func TestCmd_success(t *testing.T) {
	target := prepareServer(t)

	got, err := runCmd(t, target)
	if err != nil {
		t.Fatal("calling cmd result:", err)
	}

	t.Log("got", got)
}

func TestCmd_timeout(t *testing.T) {
	target := prepareServer(t, "sleep")

	got, err := runCmd(t, "-timeout=50ms", target)

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
}

func TestCmd_internal(t *testing.T) {
	target := prepareServer(t, "internal_error")

	got, err := runCmd(t, target)

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
}

func TestCmd_uknown_flag(t *testing.T) {
	got, err := runCmd(t, "-uknown-flag")

	t.Logf("output: %s", got)
	assertExitCode(t, err, 2)
}

func TestCmdRun(t *testing.T) {
	if os.Getenv(envCmd) != "1" {
		return
	}

	if i := slices.Index(os.Args, "--"); i > 0 {
		os.Args = os.Args[i+1:]
	}

	probes.Main()
}

func prepareServer(t *testing.T, params ...string) string {
	t.Helper()

	targetToken := strings.ToLower(rand.Text())
	called := &atomic.Bool{}

	var handle http.HandlerFunc = func(rw http.ResponseWriter, req *http.Request) {
		called.Store(true)

		t.Logf("request %s %v", req.Method, req.URL)
		if req.URL.Path != "/"+targetToken {
			t.Error("got unexpected request")
		}

		q := req.URL.Query()

		if q.Get("sleep") != "" {
			<-req.Context().Done()
		}

		if q.Get("internal_error") != "" {
			httpext.Error(rw, http.StatusInternalServerError)
		}
	}

	server := httptest.NewServer(handle)
	t.Cleanup(server.Close)

	target, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal("creating target URL", err)
	}

	target.Path = path.Join(target.Path, targetToken)
	q := target.Query()

	for _, param := range params {
		q.Add(param, "1")
	}

	target.RawQuery = q.Encode()

	return target.String()
}

func runCmd(t *testing.T, args ...string) (string, error) {
	t.Helper()

	testArgs := []string{"-test.run=TestCmdRun", "--"}
	if testing.Verbose() {
		testArgs = append(testArgs, "-v")
	}
	testArgs = append(testArgs, args...)

	cmd := exec.CommandContext(t.Context(), os.Args[0], testArgs...)
	cmd.Env = append(cmd.Env, envCmd+"=1")

	t.Log("calling:", cmd)
	output, err := cmd.CombinedOutput()

	return string(output), err
}

func assertExitCode(t *testing.T, err error, exitCode int) {
	t.Helper()

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return
	}

	got := exitErr.ExitCode()
	if got != exitCode {
		t.Errorf("want exit code %d", exitCode)
		t.Errorf(" got exit code %d", got)
	}
}
