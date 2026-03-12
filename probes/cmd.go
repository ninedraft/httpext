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
// It parses os.Args[1:], executes Cmd with flag.CommandLine,
// writes a human-readable error message to stderr on failure,
// and terminates the process via os.Exit.
//
// Use Main from a dedicated probe binary or a small main package:
//
//	func main() {
//		probes.Main()
//	}
func Main() {
	err := Cmd(flag.CommandLine, os.Args[1:])

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
	// Cmd currently accepts only 200 OK and 204 No Content as successful probe
	// responses.
	ErrBadStatus = errors.New("bad status on probe")

	// ErrTargetIsNotLoopBack reports that the target hostname resolved to one or
	// more non-loopback IP addresses.
	//
	// Cmd allows requests only to targets whose resolved addresses are all
	// loopback addresses.
	ErrTargetIsNotLoopBack = errors.New("target host must resolve only to loopback addresses")

	errBadScheme             = errors.New("bad target scheme")
	errBadHost               = errors.New("bad target host")
	errBadPort               = errors.New("bad target port")
	errRedirectsAreForbidden = errors.New("redirects are forbidden for HTTP probes")
)

// Cmd runs the HTTP probe command.
//
// Cmd defines probe-specific flags on flags, parses args, validates the target,
// resolves the target host, verifies that all resolved addresses are loopback,
// and performs the HTTP request using a transport pinned to those resolved
// loopback addresses.
//
// Security properties:
//   - only http and https targets are accepted
//   - redirects are rejected
//   - proxies are disabled
//   - the target host must resolve only to loopback addresses
//   - the actual TCP dial is pinned to the resolved loopback IPs
//
// If args does not contain a target URL, Cmd uses http://localhost:9090.
//
// The provided FlagSet must be fresh and not yet parsed, because Cmd registers
// its own flags on it and then parses args.
//
// Args must not include the program name.
//
// Typical use as a subcommand:
//
//	if len(os.Args) > 1 && os.Args[1] == "probe" {
//		fs := flag.NewFlagSet("probe", flag.ContinueOnError)
//		if err := probes.Cmd(fs, os.Args[2:]); err != nil {
//			panic("probe: " + err.Error())
//		}
//		return
//	}
func Cmd(flags *flag.FlagSet, args []string) error {
	timeout := time.Second
	flags.DurationVar(&timeout, "timeout", timeout, "request timeout")

	verbose := false
	flags.BoolVar(&verbose, "v", verbose, "print body and log interactions")

	method := http.MethodGet
	flags.StringVar(&method, "method", method, "method to call probe")

	err := flags.Parse(args)
	if errors.Is(err, flag.ErrHelp) {
		flags.Usage()
		return nil
	}

	if err != nil {
		return err
	}

	log := func(string, ...any) {}
	if verbose {
		log = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})).Debug
	}

	target := flags.Arg(0)
	if target == "" {
		target = "http://localhost:9090"

		log("got empty target, selecting default", "target", target)
	}

	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target %q: %w", target, err)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return fmt.Errorf("%w: only http and https are supported", errBadScheme)
	}
	if targetURL.Hostname() == "" {
		return fmt.Errorf("%w: empty host", errBadHost)
	}

	targetPort, err := strconv.ParseUint(targetURL.Port(), 10, 16)
	if err != nil && targetURL.Port() != "" {
		return fmt.Errorf("%w %q: %w", errBadPort, targetURL.Port(), err)
	}

	if targetPort == 0 && targetURL.Port() == "" {
		targetPort = 80
		if targetURL.Scheme == "https" {
			targetPort = 443
		}
	}

	if targetPort == 0 || targetPort > math.MaxUint16 {
		return fmt.Errorf("%w: %d is not allowed", errBadPort, targetPort)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	log("resolving target hostname", "hostname", targetURL.Hostname())
	hostAddresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", targetURL.Hostname())
	if err != nil {
		return fmt.Errorf("unable to lookup target hostname: %w", err)
	}
	if len(hostAddresses) == 0 {
		return fmt.Errorf("%w: resolver returned no addresses", errBadHost)
	}

	slices.SortStableFunc(hostAddresses, netip.Addr.Compare)

	log("checking resolved target IP addresses", "addresses", hostAddresses)
	for _, addr := range hostAddresses {
		if !addr.IsLoopback() {
			return fmt.Errorf("%w: %q -> %q", ErrTargetIsNotLoopBack, targetURL.Hostname(), hostAddresses)
		}
	}

	dialer := &net.Dialer{
		Timeout: timeout,
	}

	transport := newTransport(timeout,
		func(ctx context.Context) (conn net.Conn, err error) {
			for _, addr := range hostAddresses {
				tcpAddr := netip.AddrPortFrom(addr, uint16(targetPort))
				log("trying to dial", "address", tcpAddr)

				conn, err = dialer.DialContext(ctx, "tcp", tcpAddr.String())
				if err == nil {
					log("success!", "address", tcpAddr)
					break
				}
				if errors.Is(err, context.Canceled) ||
					errors.Is(err, context.DeadlineExceeded) {
					log("context cancelled during dial", "err", err, "err_ctx", context.Cause(ctx))
					return nil, err
				}
				log("unable to dial, trying next", "error", err)
			}

			return conn, err
		})

	client := &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return errRedirectsAreForbidden
		},
	}

	req, err := http.NewRequestWithContext(ctx, method, target, nil)
	if err != nil {
		return fmt.Errorf("preparing request: %w", err)
	}

	log("doing request", "method", method, "target", target)
	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return fmt.Errorf("making request %s %q: %w", method, req.URL, err)
	}

	log("got response", "status", resp.Status, "status_code", resp.StatusCode)

	ok := resp.StatusCode == http.StatusOK ||
		resp.StatusCode == http.StatusNoContent

	switch {
	case ok:
		fmt.Println("OK")
	default:
		fmt.Println("FAIL")
	}

	if verbose {
		fmt.Println("RESPONSE:")
		_, _ = io.Copy(os.Stdout, resp.Body)
		fmt.Println()
	}

	if !ok {
		return fmt.Errorf("%w: %d %s", ErrBadStatus, resp.StatusCode, resp.Status)
	}

	return nil
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
