package probes

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"slices"
	"strconv"
	"time"
)

// Main runs the probe CLI using the current process arguments.
//
// It parses os.Args[1:] using a fresh FlagSet,
// writes a human-readable error message to stderr on failure,
// and terminates the process via os.Exit.
//
// Flags:
//   - -timeout duration: request timeout (default 1s)
//   - -method string: HTTP method for the probe request (default GET)
//   - -v: print response body and debug log interactions
//
// Positional args:
//   - target: probe URL; if omitted, RunProbe default target is used
//
// Use Main from a dedicated probe binary or a small main package:
//
//	func main() {
//		probes.Main()
//	}
func Main() {
	flags := flag.NewFlagSet("probe", flag.ContinueOnError)
	err := runCmd(flags, os.Args[1:])

	if errors.Is(err, ErrProbeClientConfiguration) {
		fmt.Fprintf(os.Stderr, "CONFGIRATION ERROR: %v\n", err)
		os.Exit(2)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

var (
	// ErrBadStatus reports that the probe request completed successfully at the
	// transport level, but the HTTP response status code was not accepted.
	//
	// RunProbe currently accepts only 200 OK and 204 No Content as successful probe
	// responses.
	ErrBadStatus = errors.New("bad status on probe")

	// ErrTargetIsNotLoopBack reports that the target hostname resolved to one or
	// more non-loopback IP addresses.
	//
	// RunProbe allows requests only to targets whose resolved addresses are all
	// loopback addresses.
	ErrTargetIsNotLoopBack = errors.New("target host must resolve only to loopback addresses")

	// ErrConfiguration means probe client misconfigured.
	ErrProbeClientConfiguration = errors.New("probe client misconfigured")

	errBadScheme             = errors.New("bad target scheme")
	errBadHost               = errors.New("bad target host")
	errBadPort               = errors.New("bad target port")
	errRedirectsAreForbidden = errors.New("redirects are forbidden for HTTP probes")
)

func runCmd(flags *flag.FlagSet, args []string) error {
	cfg := ClientConfig{
		Timeout:     time.Second,
		Method:      http.MethodGet,
		CaptureBody: false,
		Logf:        func(msg string, args ...any) {},
	}

	flags.DurationVar(&cfg.Timeout, "timeout", cfg.Timeout, "request timeout")

	verbose := false
	flags.BoolFunc("v", "print body and log interactions",
		func(string) error {
			handlerOpts := &slog.HandlerOptions{Level: slog.LevelDebug}
			cfg.Logf = slog.New(slog.NewTextHandler(os.Stderr, handlerOpts)).Debug

			return nil
		})

	flags.StringVar(&cfg.Method, "method", cfg.Method, "method to call probe")

	err := flags.Parse(args)

	if errors.Is(err, flag.ErrHelp) {
		return nil
	}

	if err != nil {
		return errors.Join(ErrProbeClientConfiguration, err)
	}

	cfg.Target = flags.Arg(0)

	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	result, err := RunProbe(ctx, cfg)

	switch {
	case err == nil:
		fmt.Println("OK")
	case errors.Is(err, ErrBadStatus):
		fmt.Println("FAIL")
	default:
		return err
	}

	if verbose {
		fmt.Println("RESPONSE:")
		_, _ = os.Stdout.Write(result.Body)
		fmt.Println()
	}

	return err
}

// ClientConfig configures RunProbe behavior.
type ClientConfig struct {
	// Target is the probe URL.
	// If empty, RunProbe uses http://localhost:9090.
	Target string

	// Method is the HTTP request method.
	// If empty, RunProbe uses GET.
	Method string

	// Timeout is used for transport-level timeouts and HTTP client timeout.
	// If zero or negative, RunProbe uses 1 second.
	Timeout time.Duration

	// CaptureBody enables reading response body into ProbeResult.Body.
	CaptureBody bool

	// Logf receives debug log events.
	// If nil, logging is disabled.
	Logf func(msg string, args ...any)
}

// ProbeResult is the HTTP result of RunProbe.
type ProbeResult struct {
	StatusCode int
	Status     string
	Body       []byte
}

// RunProbe executes a single HTTP probe request.
//
// Security properties:
//   - only http and https targets are accepted
//   - redirects are rejected
//   - proxies are disabled
//   - the target host must resolve only to loopback addresses
//   - the actual TCP dial is pinned to the resolved loopback IPs
//
// For response status 200 OK and 204 No Content, RunProbe returns nil error.
// For any other response status, RunProbe returns ErrBadStatus and a populated
// ProbeResult.
func RunProbe(ctx context.Context, cfg ClientConfig) (ProbeResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	cfg = withProbeConfigDefaults(cfg)

	ctx, cancel := context.WithTimeout(ctx, cfg.Timeout)
	defer cancel()

	targetURL, targetPort, err := parseProbeTarget(cfg.Target)
	if err != nil {
		return ProbeResult{}, err
	}

	hostAddresses, err := resolveProbeTargetAddresses(ctx, targetURL.Hostname(), cfg.Logf)
	if err != nil {
		return ProbeResult{}, err
	}

	client := newProbeClient(cfg.Timeout, hostAddresses, targetPort, cfg.Logf)

	req, err := http.NewRequestWithContext(ctx, cfg.Method, cfg.Target, nil)
	if err != nil {
		return ProbeResult{}, fmt.Errorf("preparing request: %w", err)
	}

	cfg.Logf("doing request", "method", cfg.Method, "target", cfg.Target)
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return ProbeResult{}, fmt.Errorf("making request %s %q: %w", cfg.Method, req.URL, err)
	}

	cfg.Logf("got response", "status", resp.Status, "status_code", resp.StatusCode)

	result := ProbeResult{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
	}

	if cfg.CaptureBody {
		result.Body, err = io.ReadAll(resp.Body)
		if err != nil {
			return result, fmt.Errorf("reading response body: %w", err)
		}
	}

	ok := result.StatusCode == http.StatusOK ||
		result.StatusCode == http.StatusNoContent
	if !ok {
		return result, fmt.Errorf("%w: %d %s", ErrBadStatus, result.StatusCode, result.Status)
	}

	return result, nil
}

func withProbeConfigDefaults(cfg ClientConfig) ClientConfig {
	if cfg.Logf == nil {
		cfg.Logf = func(string, ...any) {}
	}
	if cfg.Target == "" {
		cfg.Target = "http://localhost:9090"
		cfg.Logf("got empty target, selecting default", "target", cfg.Target)
	}
	if cfg.Method == "" {
		cfg.Method = http.MethodGet
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}

	return cfg
}

func parseProbeTarget(target string) (*url.URL, uint16, error) {
	targetURL, err := url.Parse(target)
	if err != nil {
		return nil, 0, fmt.Errorf("%w, invalid target URL %q: %w", ErrProbeClientConfiguration, target, err)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return nil, 0, fmt.Errorf("%w, %w: only http and https are supported, got %q", ErrProbeClientConfiguration, errBadScheme, targetURL.Scheme)
	}
	if targetURL.Hostname() == "" {
		return nil, 0, fmt.Errorf("%w, %w: empty host", ErrProbeClientConfiguration, errBadHost)
	}

	targetPort, err := strconv.ParseUint(targetURL.Port(), 10, 16)
	if err != nil && targetURL.Port() != "" {
		return nil, 0, fmt.Errorf("%w, %w %q: %w", ErrProbeClientConfiguration, errBadPort, targetURL.Port(), err)
	}
	if targetPort == 0 && targetURL.Port() == "" {
		targetPort = 80
		if targetURL.Scheme == "https" {
			targetPort = 443
		}
	}
	if targetPort == 0 || targetPort > math.MaxUint16 {
		return nil, 0, fmt.Errorf("%w, %w: %d is not allowed", ErrProbeClientConfiguration, errBadPort, targetPort)
	}

	return targetURL, uint16(targetPort), nil
}

func resolveProbeTargetAddresses(ctx context.Context, hostname string, logf func(string, ...any)) ([]netip.Addr, error) {
	logf("resolving target hostname", "hostname", hostname)
	hostAddresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", hostname)
	if err != nil {
		return nil, fmt.Errorf("unable to lookup target hostname: %w", err)
	}
	if len(hostAddresses) == 0 {
		return nil, fmt.Errorf("%w: resolver returned no addresses", errBadHost)
	}

	slices.SortFunc(hostAddresses, netip.Addr.Compare)

	logf("checking resolved target IP addresses", "addresses", hostAddresses)
	for _, addr := range hostAddresses {
		if !addr.IsLoopback() {
			return nil, fmt.Errorf("%w: %q -> %q", ErrTargetIsNotLoopBack, hostname, hostAddresses)
		}
	}

	return hostAddresses, nil
}

func newProbeClient(timeout time.Duration, hostAddresses []netip.Addr, targetPort uint16, logf func(string, ...any)) *http.Client {
	dialer := &net.Dialer{
		Timeout: timeout,
	}

	transport := newTransport(timeout,
		func(ctx context.Context) (net.Conn, error) {
			var lastErr error

			for _, addr := range hostAddresses {
				tcpAddr := netip.AddrPortFrom(addr, targetPort)
				logf("trying to dial", "address", tcpAddr)

				conn, err := dialer.DialContext(ctx, "tcp", tcpAddr.String())
				if err == nil {
					logf("success!", "address", tcpAddr)
					return conn, nil
				}
				if errors.Is(err, context.Canceled) ||
					errors.Is(err, context.DeadlineExceeded) {
					logf("context cancelled during dial", "err", err, "err_ctx", context.Cause(ctx))
					return nil, err
				}

				lastErr = err
				logf("unable to dial, trying next", "error", err)
			}

			return nil, lastErr
		})

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirectsAreForbidden
		},
	}
}

func newTransport(timeout time.Duration, dial func(ctx context.Context) (net.Conn, error)) *http.Transport {
	return &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return dial(ctx)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          1,
		IdleConnTimeout:       timeout,
		TLSHandshakeTimeout:   timeout,
		ExpectContinueTimeout: timeout,
		ResponseHeaderTimeout: timeout,
	}
}
