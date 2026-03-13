package probes_test

import (
	"context"
	"crypto/rand"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"os/exec"
	"path"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ninedraft/httpext"
	"github.com/ninedraft/httpext/probes"
)

const envCmd = "GO_WANT_HELPER_PROCESS"

func TestCmd_success(t *testing.T) {
	t.Parallel()

	target := prepareServer(t)

	got, err := runCmd(t, target)
	if err != nil {
		t.Fatal("calling cmd result:", err)
	}
	assertOutputContains(t, got, "OK")

	t.Log("got", got)
}

func TestCmd_timeout(t *testing.T) {
	t.Parallel()

	target := prepareServer(t, "sleep")

	got, err := runCmd(t, "-timeout=50ms", target)

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
	assertOutputContains(t, got,
		"timeout awaiting response headers",
		"context deadline exceeded",
	)
}

func TestCmd_internal(t *testing.T) {
	t.Parallel()

	target := prepareServer(t, "internal_error")

	got, err := runCmd(t, target)

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
	assertOutputContains(t, got, "FAIL")
	assertOutputContains(t, got, probes.ErrClientProbeBadStatus.Error())
}

func TestCmd_uknown_flag(t *testing.T) {
	t.Parallel()

	got, err := runCmd(t, "-uknown-flag")

	t.Logf("output: %s", got)
	assertExitCode(t, err, 2)
	assertOutputContains(t, got, "CONFGIRATION ERROR")
}

func TestCmd_help(t *testing.T) {
	t.Parallel()

	for _, arg := range []string{"-h", "--help"} {
		t.Run(arg, func(t *testing.T) {
			t.Parallel()

			got, err := runCmd(t, arg)

			t.Logf("output: %s", got)
			assertExitCode(t, err, 0)
			assertOutputContains(t, got, "Usage of probe:")
		})
	}
}

func TestCmd_configuration_validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		target      string
		wantInOutpt string
	}{
		{
			name:        "bad scheme",
			target:      "ftp://localhost:9090",
			wantInOutpt: "bad target scheme",
		},
		{
			name:        "empty host",
			target:      "http:///path",
			wantInOutpt: "empty host",
		},
		{
			name:        "bad port",
			target:      "http://localhost:abc",
			wantInOutpt: "invalid target URL",
		},
		{
			name:        "port zero",
			target:      "http://localhost:0",
			wantInOutpt: "bad target port",
		},
		{
			name:        "port out of range",
			target:      "http://localhost:65536",
			wantInOutpt: "bad target port",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := runCmd(t, test.target)

			t.Logf("output: %s", got)
			assertExitCode(t, err, 2)
			assertOutputContains(t, got, "CONFGIRATION ERROR")
			assertOutputContains(t, got, test.wantInOutpt)
		})
	}
}

func TestCmd_non_loopback_target_rejected(t *testing.T) {
	t.Parallel()

	got, err := runCmd(t, "http://1.1.1.1:80")

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
	assertOutputContains(t, got, probes.ErrTargetIsNotLoopBack.Error())
}

func TestCmd_redirect_forbidden(t *testing.T) {
	t.Parallel()

	target := prepareServer(t, "redirect")

	got, err := runCmd(t, target)

	t.Logf("output: %s", got)
	assertExitCode(t, err, 1)
	assertOutputContains(t, got, "redirects are forbidden for HTTP probes")
}

func TestCmd_no_content_is_success(t *testing.T) {
	t.Parallel()

	target := prepareServer(t, "no_content")

	got, err := runCmd(t, target)
	if err != nil {
		t.Fatalf("calling cmd result: %v\noutput: %s", err, got)
	}

	t.Logf("output: %s", got)
	assertOutputContains(t, got, "OK")
}

func TestCmd_post_method(t *testing.T) {
	t.Parallel()

	target := prepareServer(t, "expect_post")

	got, err := runCmd(t, "-method=POST", target)
	if err != nil {
		t.Fatalf("calling cmd result: %v\noutput: %s", err, got)
	}

	t.Logf("output: %s", got)
	assertOutputContains(t, got, "OK")
}

func TestCmd_default_target(t *testing.T) {
	t.Parallel()

	var called atomic.Bool
	serveDefaultTarget(t, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		called.Store(true)
		rw.WriteHeader(http.StatusNoContent)
	}))

	got, err := runCmd(t, "-v", "-timeout=150ms")
	if err != nil {
		t.Fatalf("calling cmd result: %v\noutput: %s", err, got)
	}

	t.Logf("output: %s", got)
	if !called.Load() {
		t.Fatal("expected default target server to be called")
	}
	assertOutputContains(t, got, "got empty target, selecting default")
	assertOutputContains(t, got, "OK")
}

func TestRunProbe_success(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		target         string
		wantStatusCode int
	}{
		{
			name:           "status ok",
			target:         prepareServer(t),
			wantStatusCode: http.StatusOK,
		},
		{
			name:           "status no content",
			target:         prepareServer(t, "no_content"),
			wantStatusCode: http.StatusNoContent,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			result, err := probes.RunProbe(t.Context(), probes.ClientConfig{
				Target: test.target,
			})
			if err != nil {
				t.Fatalf("calling RunProbe: %v", err)
			}

			if result.StatusCode != test.wantStatusCode {
				t.Fatalf("want status code %d, got %d", test.wantStatusCode, result.StatusCode)
			}
		})
	}
}

func TestRunProbe_configuration_validation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		target  string
		wantErr string
	}{
		{
			name:    "bad scheme",
			target:  "ftp://localhost:9090",
			wantErr: "bad target scheme",
		},
		{
			name:    "empty host",
			target:  "http:///path",
			wantErr: "empty host",
		},
		{
			name:    "bad port",
			target:  "http://localhost:abc",
			wantErr: "invalid target URL",
		},
		{
			name:    "port zero",
			target:  "http://localhost:0",
			wantErr: "bad target port",
		},
		{
			name:    "port out of range",
			target:  "http://localhost:65536",
			wantErr: "bad target port",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := probes.RunProbe(t.Context(), probes.ClientConfig{
				Target: test.target,
			})

			if !errors.Is(err, probes.ErrClientProbeConfiguration) {
				t.Fatalf("want ErrProbeClientConfiguration, got: %v", err)
			}
			if !strings.Contains(err.Error(), test.wantErr) {
				t.Fatalf("want %q in error, got: %v", test.wantErr, err)
			}
		})
	}
}

func TestRunProbe_non_loopback_target_rejected(t *testing.T) {
	t.Parallel()

	_, err := probes.RunProbe(t.Context(), probes.ClientConfig{
		Target: "http://1.1.1.1:80",
	})
	if !errors.Is(err, probes.ErrTargetIsNotLoopBack) {
		t.Fatalf("want ErrTargetIsNotLoopBack, got: %v", err)
	}
}

func TestRunProbe_redirect_forbidden(t *testing.T) {
	t.Parallel()

	_, err := probes.RunProbe(t.Context(), probes.ClientConfig{
		Target: prepareServer(t, "redirect"),
	})
	if err == nil {
		t.Fatal("want error, got nil")
	}
	if !strings.Contains(err.Error(), "redirects are forbidden for HTTP probes") {
		t.Fatalf("want redirect error, got: %v", err)
	}
}

func TestRunProbe_bad_status_returns_result(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte("boom"))
	}))
	t.Cleanup(server.Close)

	result, err := probes.RunProbe(t.Context(), probes.ClientConfig{
		Target:      server.URL,
		CaptureBody: true,
	})
	if !errors.Is(err, probes.ErrClientProbeBadStatus) {
		t.Fatalf("want ErrBadStatus, got: %v", err)
	}

	if result.StatusCode != http.StatusInternalServerError {
		t.Fatalf("want status %d, got %d", http.StatusInternalServerError, result.StatusCode)
	}
	if result.Status != "500 Internal Server Error" {
		t.Fatalf("want status text %q, got %q", "500 Internal Server Error", result.Status)
	}
	if string(result.Body) != "boom" {
		t.Fatalf("want body %q, got %q", "boom", string(result.Body))
	}
}

func TestRunProbe_capture_body_toggle(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		_, _ = rw.Write([]byte("healthy"))
	}))
	t.Cleanup(server.Close)

	withBody, err := probes.RunProbe(t.Context(), probes.ClientConfig{
		Target:      server.URL,
		CaptureBody: true,
	})
	if err != nil {
		t.Fatalf("capture body enabled: %v", err)
	}
	if string(withBody.Body) != "healthy" {
		t.Fatalf("want body %q, got %q", "healthy", string(withBody.Body))
	}

	withoutBody, err := probes.RunProbe(t.Context(), probes.ClientConfig{
		Target:      server.URL,
		CaptureBody: false,
	})
	if err != nil {
		t.Fatalf("capture body disabled: %v", err)
	}
	if len(withoutBody.Body) != 0 {
		t.Fatalf("want empty body, got %q", string(withoutBody.Body))
	}
}

func TestCmdRun(t *testing.T) {
	if os.Getenv(envCmd) != "1" {
		return
	}

	if i := slices.Index(os.Args, "--"); i > 0 {
		// drop -test.run=TestCmdRun -- part
		os.Args = slices.Delete(os.Args, 1, i+1)
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
			return
		}

		if q.Get("redirect") != "" {
			http.Redirect(rw, req, "/redirected", http.StatusFound)
			return
		}

		if q.Get("no_content") != "" {
			rw.WriteHeader(http.StatusNoContent)
			return
		}

		if q.Get("expect_post") != "" && req.Method != http.MethodPost {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
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

	if exitCode == 0 {
		if err != nil {
			t.Fatalf("want exit code %d, got error %v", exitCode, err)
		}

		return
	}

	if err == nil {
		t.Fatalf("want exit code %d, got nil error", exitCode)
	}

	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("want *exec.ExitError, got %T: %v", err, err)
	}

	got := exitErr.ExitCode()
	if got != exitCode {
		t.Errorf("want exit code %d", exitCode)
		t.Errorf(" got exit code %d", got)
	}
}

func assertOutputContains(t *testing.T, got string, want ...string) {
	t.Helper()

	for _, candidate := range want {
		if strings.Contains(got, candidate) {
			return
		}
	}

	t.Fatalf("want output to contain one of %q, got:\n%s", want, got)
}

func serveDefaultTarget(t *testing.T, handler http.Handler) {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), time.Second)
	defer cancel()

	addrs, err := net.DefaultResolver.LookupNetIP(ctx, "ip", "localhost")
	if err != nil {
		t.Skipf("unable to resolve localhost: %v", err)
	}
	if len(addrs) == 0 {
		t.Skip("localhost resolver returned no addresses")
	}

	hosts := make(map[netip.Addr]struct{}, len(addrs))
	for _, addr := range addrs {
		hosts[addr.Unmap()] = struct{}{}
	}

	for host := range hosts {
		listener, err := net.Listen("tcp", netip.AddrPortFrom(host, 9090).String())
		if err != nil {
			t.Skipf("unable to bind %s:9090: %v", host, err)
		}

		server := httptest.NewUnstartedServer(handler)
		server.Listener = listener
		server.Start()
		t.Cleanup(server.Close)
	}
}
