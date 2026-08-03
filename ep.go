package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	banner = `
  _____   _ 
 / _ \ \ / /
|  __/  V / 
 \___|_|_\  
`

	devName        = "INTELEON404"
	version        = "v1"
	defaultTimeout = 10
	defaultThreads = 20
	maxBodySize    = 10 * 1024 * 1024 // 10MB
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type Options struct {
	URL            string
	Input          string
	Wordlist       string
	Proxy          string
	Method         string
	Data           string
	Headers        stringSlice
	Threads        int
	Timeout        int
	Delay          int
	MaxBodySize    int64
	IgnoreCodes    string
	MatchCodes     string
	Insecure       bool
	FollowRedirect bool
	Silent         bool
	Verbose        bool
	ShowStatusCode bool
	ShowContentLen bool
	ShowRespTime   bool
	ShowServer     bool
	UserAgent      string
	Output         string
	Retries        int
	ignoreList     []int
	matchList      []int
}

type Result struct {
	URL        string
	Method     string
	StatusCode int
	ContentLen int64
	RespTime   time.Duration
	Server     string
}

type Job struct {
	Base string
	Path string
}

func showHelp() {
	fmt.Print(banner)
	fmt.Printf("  %s %s by %s\n\n", "ep", version, devName)
	fmt.Println("Usage: ep -u <URL> -w <WORDLIST> [FLAGS]")
	fmt.Println("       ep -i <URL_LIST> -w <WORDLIST> [FLAGS]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -u, --url string          Target base URL")
	fmt.Println("  -i, --input string        File containing target URLs")
	fmt.Println("  -w, --wordlist string     Path to wordlist file")
	fmt.Println("  -m, --method string       HTTP method (default GET)")
	fmt.Println("  -d, --data string         Request body for POST/PUT/PATCH")
	fmt.Println("  -H, --header string       Custom headers (repeatable)")
	fmt.Println("  -p, --proxy string        Proxy URL")
	fmt.Println("  -t, --threads int         Concurrent threads (default 20)")
	fmt.Println("  --delay int               Delay between requests in ms (default 0)")
	fmt.Println("  --timeout int             Request timeout in seconds (default 10)")
	fmt.Println("  --max-size int            Max response body in bytes (default 10485760)")
	fmt.Println("  --ignore-code string      Comma-separated status codes to ignore (default 404)")
	fmt.Println("  -mc, --match-code string  Comma-separated status codes to match")
	fmt.Println("  --retries int             Max retries on failure (default 3)")
	fmt.Println("  --insecure                Skip TLS certificate verification")
	fmt.Println("  --follow-redirects        Follow HTTP redirects (default false)")
	fmt.Println("  --user-agent string       Custom User-Agent")
	fmt.Println("  -sc, --status-code        Show status code (default true)")
	fmt.Println("  -cl, --content-length     Show content length (default true)")
	fmt.Println("  -rt, --response-time      Show response time")
	fmt.Println("  -server                   Show Server header")
	fmt.Println("  -o, --output string       Save results to file")
	fmt.Println("  -silent                   Output only discovered endpoints")
	fmt.Println("  -v, --verbose             Show error details")
	fmt.Println("  -h, --help                Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ep -u https://api.example.com -w endpoints.txt")
	fmt.Println("  ep -i urls.txt -w endpoints.txt -mc 200,301,302")
	fmt.Println("  ep -u https://api.example.com -w endpoints.txt -m POST -d '{\"key\":\"val\"}' -H 'Content-Type: application/json'")
}

func parseFlags() *Options {
	opts := &Options{}

	flag.StringVar(&opts.URL, "u", "", "Target base URL")
	flag.StringVar(&opts.URL, "url", "", "Target base URL")
	flag.StringVar(&opts.Input, "i", "", "File containing target URLs")
	flag.StringVar(&opts.Input, "input", "", "File containing target URLs")
	flag.StringVar(&opts.Wordlist, "w", "", "Path to wordlist file")
	flag.StringVar(&opts.Wordlist, "wordlist", "", "Path to wordlist file")
	flag.StringVar(&opts.Method, "m", "GET", "HTTP method")
	flag.StringVar(&opts.Method, "method", "GET", "HTTP method")
	flag.StringVar(&opts.Data, "d", "", "Request body")
	flag.StringVar(&opts.Data, "data", "", "Request body")
	flag.StringVar(&opts.Proxy, "p", "", "Proxy URL")
	flag.StringVar(&opts.Proxy, "proxy", "", "Proxy URL")
	flag.Var(&opts.Headers, "H", "Custom header")
	flag.Var(&opts.Headers, "header", "Custom header")
	flag.IntVar(&opts.Threads, "t", defaultThreads, "Concurrent threads")
	flag.IntVar(&opts.Threads, "threads", defaultThreads, "Concurrent threads")
	flag.IntVar(&opts.Delay, "delay", 0, "Delay between requests in ms")
	flag.IntVar(&opts.Timeout, "timeout", defaultTimeout, "Request timeout in seconds")
	flag.Int64Var(&opts.MaxBodySize, "max-size", maxBodySize, "Max response body in bytes")
	flag.StringVar(&opts.IgnoreCodes, "ignore-code", "404", "Status codes to ignore")
	flag.StringVar(&opts.MatchCodes, "mc", "", "Status codes to match")
	flag.StringVar(&opts.MatchCodes, "match-code", "", "Status codes to match")
	flag.IntVar(&opts.Retries, "retries", 3, "Max retries on failure")
	flag.BoolVar(&opts.Insecure, "insecure", false, "Skip TLS verification")
	flag.BoolVar(&opts.FollowRedirect, "follow-redirects", false, "Follow redirects")
	flag.StringVar(&opts.UserAgent, "user-agent", "", "Custom User-Agent")
	flag.BoolVar(&opts.ShowStatusCode, "sc", true, "Show status code")
	flag.BoolVar(&opts.ShowStatusCode, "status-code", true, "Show status code")
	flag.BoolVar(&opts.ShowContentLen, "cl", true, "Show content length")
	flag.BoolVar(&opts.ShowContentLen, "content-length", true, "Show content length")
	flag.BoolVar(&opts.ShowRespTime, "rt", false, "Show response time")
	flag.BoolVar(&opts.ShowRespTime, "response-time", false, "Show response time")
	flag.BoolVar(&opts.ShowServer, "server", false, "Show Server header")
	flag.StringVar(&opts.Output, "o", "", "Save results to file")
	flag.StringVar(&opts.Output, "output", "", "Save results to file")
	flag.BoolVar(&opts.Silent, "silent", false, "Silent mode")
	flag.BoolVar(&opts.Verbose, "v", false, "Verbose output")
	flag.BoolVar(&opts.Verbose, "verbose", false, "Verbose output")

	help := flag.Bool("h", false, "Show help")
	flag.BoolVar(help, "help", false, "Show help")
	flag.Usage = func() { showHelp() }
	flag.Parse()

	if *help || ((opts.URL == "" && opts.Input == "") || opts.Wordlist == "") {
		showHelp()
		os.Exit(0)
	}

	opts.Method = strings.ToUpper(opts.Method)
	if opts.UserAgent == "" {
		opts.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
	}

	for _, codeStr := range strings.Split(opts.IgnoreCodes, ",") {
		if code, err := strconv.Atoi(strings.TrimSpace(codeStr)); err == nil {
			opts.ignoreList = append(opts.ignoreList, code)
		}
	}

	if opts.MatchCodes != "" {
		for _, codeStr := range strings.Split(opts.MatchCodes, ",") {
			if code, err := strconv.Atoi(strings.TrimSpace(codeStr)); err == nil {
				opts.matchList = append(opts.matchList, code)
			}
		}
	}

	return opts
}

func shouldIgnore(code int, list []int) bool {
	for _, ic := range list {
		if code == ic {
			return true
		}
	}
	return false
}

func shouldMatch(code int, list []int) bool {
	if len(list) == 0 {
		return true
	}
	for _, mc := range list {
		if code == mc {
			return true
		}
	}
	return false
}

func buildClient(opts *Options) *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: opts.Insecure},
	}

	if opts.Proxy != "" {
		proxyURL, err := url.Parse(opts.Proxy)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] Invalid proxy URL: %v\n", err)
			os.Exit(1)
		}
		tr.Proxy = http.ProxyURL(proxyURL)
	}

	checkRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	if opts.FollowRedirect {
		checkRedirect = nil
	}

	return &http.Client{
		Transport:     tr,
		Timeout:       time.Duration(opts.Timeout) * time.Second,
		CheckRedirect: checkRedirect,
	}
}

func doRequest(ctx context.Context, client *http.Client, opts *Options, target string) (*http.Response, error) {
	var bodyReader io.Reader
	if opts.Data != "" {
		bodyReader = strings.NewReader(opts.Data)
	}

	req, err := http.NewRequestWithContext(ctx, opts.Method, target, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Accept", "*/*")

	for _, h := range opts.Headers {
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			req.Header.Set(strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]))
		}
	}

	return client.Do(req)
}

func worker(ctx context.Context, client *http.Client, opts *Options, jobs <-chan Job, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for job := range jobs {
		if opts.Delay > 0 {
			time.Sleep(time.Duration(opts.Delay) * time.Millisecond)
		}

		target, err := url.JoinPath(job.Base, job.Path)
		if err != nil {
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[ERR] Bad URL join: %s + %s | %v\n", job.Base, job.Path, err)
			}
			continue
		}

		var resp *http.Response
		var lastErr error
		start := time.Now()

		for attempt := 0; attempt <= opts.Retries; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(attempt) * time.Second
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[RETRY] %s (attempt %d/%d, backoff %s)\n", target, attempt, opts.Retries, backoff)
				}
				time.Sleep(backoff)
			}

			reqCtx, cancel := context.WithTimeout(ctx, time.Duration(opts.Timeout)*time.Second)
			resp, lastErr = doRequest(reqCtx, client, opts, target)
			cancel()

			if lastErr != nil {
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[ERR] %s | %v\n", target, lastErr)
				}
				continue
			}

			if resp.StatusCode == 429 {
				resp.Body.Close()
				if attempt < opts.Retries {
					continue
				}
			}
			break
		}

		respTime := time.Since(start)

		if lastErr != nil {
			continue
		}
		if resp == nil {
			continue
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, opts.MaxBodySize))
		resp.Body.Close()

		if err != nil && opts.Verbose {
			fmt.Fprintf(os.Stderr, "[ERR] Reading body: %s | %v\n", target, err)
		}

		server := resp.Header.Get("Server")

		if !shouldIgnore(resp.StatusCode, opts.ignoreList) && shouldMatch(resp.StatusCode, opts.matchList) {
			results <- Result{
				URL:        target,
				Method:     opts.Method,
				StatusCode: resp.StatusCode,
				ContentLen: int64(len(body)),
				RespTime:   respTime,
				Server:     server,
			}
		}
	}
}

func formatResult(r Result, opts *Options) string {
	if opts.Silent {
		return r.URL
	}

	var parts []string

	if opts.ShowStatusCode {
		color := "\033[32m"
		switch {
		case r.StatusCode >= 300 && r.StatusCode < 400:
			color = "\033[33m"
		case r.StatusCode >= 400 && r.StatusCode < 500:
			color = "\033[31m"
		case r.StatusCode >= 500:
			color = "\033[35m"
		}
		reset := "\033[0m"
		parts = append(parts, fmt.Sprintf("[%s%d%s]", color, r.StatusCode, reset))
	}

	parts = append(parts, fmt.Sprintf("[%s]", r.Method), r.URL)

	if opts.ShowContentLen {
		parts = append(parts, fmt.Sprintf("[Size: %d]", r.ContentLen))
	}

	if opts.ShowRespTime {
		parts = append(parts, fmt.Sprintf("[Time: %s]", r.RespTime.Round(time.Millisecond)))
	}

	if opts.ShowServer && r.Server != "" {
		parts = append(parts, fmt.Sprintf("[Server: %s]", r.Server))
	}

	return strings.Join(parts, " ")
}

func main() {
	opts := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var targets []string
	if opts.URL != "" {
		targets = append(targets, opts.URL)
	}
	if opts.Input != "" {
		inFile, err := os.Open(opts.Input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] Could not open URL list: %v\n", err)
			os.Exit(1)
		}
		scanner := bufio.NewScanner(inFile)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			targets = append(targets, line)
		}
		inFile.Close()
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] Reading URL list: %v\n", err)
			os.Exit(1)
		}
	}

	if len(targets) == 0 {
		showHelp()
		os.Exit(0)
	}

	file, err := os.Open(opts.Wordlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Could not open wordlist: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	var paths []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		paths = append(paths, line)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Reading wordlist: %v\n", err)
		os.Exit(1)
	}

	client := buildClient(opts)

	jobs := make(chan Job, opts.Threads*2)
	results := make(chan Result, opts.Threads*2)
	var wg sync.WaitGroup

	for i := 0; i < opts.Threads; i++ {
		wg.Add(1)
		go worker(ctx, client, opts, jobs, results, &wg)
	}

	var outFile *os.File
	if opts.Output != "" {
		outFile, err = os.Create(opts.Output)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[FATAL] Could not create output file: %v\n", err)
			os.Exit(1)
		}
		defer outFile.Close()
	}

	go func() {
		for r := range results {
			line := formatResult(r, opts)
			fmt.Println(line)

			if outFile != nil {
				if opts.Silent {
					fmt.Fprintln(outFile, r.URL)
				} else {
					plain := fmt.Sprintf("[%d] [%s] %s [Size: %d]", r.StatusCode, r.Method, r.URL, r.ContentLen)
					if opts.ShowRespTime {
						plain += fmt.Sprintf(" [Time: %s]", r.RespTime.Round(time.Millisecond))
					}
					if opts.ShowServer && r.Server != "" {
						plain += fmt.Sprintf(" [Server: %s]", r.Server)
					}
					fmt.Fprintln(outFile, plain)
				}
			}
		}
	}()

	if !opts.Silent {
		fmt.Print(banner)
		fmt.Printf("  %s %s by %s\n", "ep", version, devName)
		fmt.Println("  Advanced API Endpoint Discovery Engine")
		fmt.Println()
		fmt.Printf("[*] Targets  : %d\n", len(targets))
		if opts.URL != "" {
			fmt.Printf("[*] Base URL : %s\n", opts.URL)
		}
		if opts.Input != "" {
			fmt.Printf("[*] URL List : %s\n", opts.Input)
		}
		fmt.Printf("[*] Wordlist : %s\n", opts.Wordlist)
		fmt.Printf("[*] Method   : %s\n", opts.Method)
		if opts.Proxy != "" {
			fmt.Printf("[*] Proxy    : %s\n", opts.Proxy)
		}
		if len(opts.Headers) > 0 {
			fmt.Printf("[*] Headers  : %d custom header(s)\n", len(opts.Headers))
		}
		fmt.Printf("[*] Threads  : %d | Delay: %dms | Timeout: %ds | Retries: %d\n", opts.Threads, opts.Delay, opts.Timeout, opts.Retries)
		fmt.Printf("[*] Ignore   : %v\n", opts.ignoreList)
		if len(opts.matchList) > 0 {
			fmt.Printf("[*] Match    : %v\n", opts.matchList)
		}
		if opts.Insecure {
			fmt.Println("[!] Warning  : TLS verification disabled")
		}
		fmt.Println("--------------------------------------------------")
	}

	for _, target := range targets {
		for _, path := range paths {
			select {
			case <-ctx.Done():
				goto done
			default:
			}
			jobs <- Job{Base: target, Path: path}
		}
	}

done:
	close(jobs)
	wg.Wait()
	close(results)

	time.Sleep(100 * time.Millisecond)
}
