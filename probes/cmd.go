package probes

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"time"
)

// Main calls Cmd util as main comman for your program.
// Main always calls os.Exit.
//
// Use it in main package to create a cmd util.
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
	ErrBadStatus           = errors.New("bad status on probe")
	ErrTargetIsNotLoopBack = errors.New("only loopback targets are allowed!")

	errBadScheme             = errors.New("bad target scheme")
	errBadPort               = errors.New("bad target port")
	errRedirectsAreForbidden = errors.New("redirects are forbidden for HTTP probes")
)

// Cmd allows to build an embedded HTTP prober for your app.
//
// Ship your apps using scratch images!
//
// Cmd will allow to dial only local addresses.
//
// Call it as a subcommand
//
//	if flag.Arg(0) == "probe" {
//		flagset := flag.NewFlagSet("probe", flag.ContinueOnError)
//		err := probes.Cmd(flagset, os.Args[2:])
//		if err != nil { panic(err) }
//		return
//	 }
//
// or just make a separate main package
//
//	func main() {
//	   probes.Main()
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

	target := flags.Arg(0)
	if target == "" {
		target = "http://localhost:9090"
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	targetURL, err := url.Parse(target)
	if err != nil {
		return fmt.Errorf("invalid target %q: %w", target, err)
	}
	if targetURL.Scheme != "http" && targetURL.Scheme != "https" {
		return fmt.Errorf("%w: only http and https are supported", errBadScheme)
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

	if targetPort == 0 {
		return fmt.Errorf("%w: zero is not allowed", errBadPort)
	}

	hostAddresses, err := net.DefaultResolver.LookupNetIP(ctx, "ip", targetURL.Hostname())
	if err != nil {
		return fmt.Errorf("unable to lookup target hostname: %w", err)
	}

	var loopbacks []netip.Addr
	for _, addr := range hostAddresses {
		if addr.IsLoopback() {
			loopbacks = append(loopbacks, addr)
		}
	}
	if len(loopbacks) == 0 {
		return fmt.Errorf("%w: %q -> %q", ErrTargetIsNotLoopBack, targetURL.Hostname(), hostAddresses)
	}

	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.ResponseHeaderTimeout = timeout

	dialer := &net.Dialer{}

	transport.DialContext = func(ctx context.Context, network, addr string) (conn net.Conn, err error) {
		for _, addr := range loopbacks {
			conn, err = dialer.DialContext(ctx, "tcp", (&net.TCPAddr{
				IP:   addr.AsSlice(),
				Zone: addr.Zone(),
				Port: int(targetPort),
			}).String())
			if err == nil {
				break
			}
		}

		return conn, err
	}

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

	resp, err := client.Do(req)
	if resp != nil {
		defer resp.Body.Close()
	}

	if err != nil {
		return fmt.Errorf("making request %s %q: %w", method, req.URL, err)
	}

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
