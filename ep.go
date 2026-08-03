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

	devName        = "INTELEON"
	version        = "v1.0"
	defaultTimeout = 10
	defaultThreads = 20
	maxBodySize    = 10 * 1024 * 1024 // 10MB
	maxRetries     = 3
)

type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

type Options struct {
	URL            string
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
	Insecure       bool
	FollowRedirect bool
	Silent         bool
	Verbose        bool
	UserAgent      string
	ignoreList     []int
}

type Result struct {
	URL        string
	Method     string
	StatusCode int
	ContentLen int64
}

func showHelp() {
	fmt.Print(banner)
	fmt.Printf("  %s %s by %s\n\n", "ep", version, devName)
	fmt.Println("Usage: ep -u <URL> -w <WORDLIST> [FLAGS]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -u, --url string          Target base URL")
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
	fmt.Println("  --insecure                Skip TLS certificate verification")
	fmt.Println("  --follow-redirects        Follow HTTP redirects (default false)")
	fmt.Println("  --user-agent string       Custom User-Agent")
	fmt.Println("  -silent                   Output only discovered endpoints")
	fmt.Println("  -v, --verbose             Show error details")
	fmt.Println("  -h, --help                Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  ep -u https://api.example.com -w endpoints.txt")
	fmt.Println("  ep -u https://api.example.com -w endpoints.txt -m POST -d '{\"key\":\"val\"}' -H 'Content-Type: application/json'")
}

func parseFlags() *Options {
	opts := &Options{}

	flag.StringVar(&opts.URL, "u", "", "")
	flag.StringVar(&opts.URL, "url", "", "")
	flag.StringVar(&opts.Wordlist, "w", "", "")
	flag.StringVar(&opts.Wordlist, "wordlist", "", "")
	flag.StringVar(&opts.Method, "m", "GET", "")
	flag.StringVar(&opts.Method, "method", "GET", "")
	flag.StringVar(&opts.Data, "d", "", "")
	flag.StringVar(&opts.Data, "data", "", "")
	flag.StringVar(&opts.Proxy, "p", "", "")
	flag.StringVar(&opts.Proxy, "proxy", "", "")
	flag.Var(&opts.Headers, "H", "")
	flag.Var(&opts.Headers, "header", "")
	flag.IntVar(&opts.Threads, "t", defaultThreads, "")
	flag.IntVar(&opts.Threads, "threads", defaultThreads, "")
	flag.IntVar(&opts.Delay, "delay", 0, "")
	flag.IntVar(&opts.Timeout, "timeout", defaultTimeout, "")
	flag.Int64Var(&opts.MaxBodySize, "max-size", maxBodySize, "")
	flag.StringVar(&opts.IgnoreCodes, "ignore-code", "404", "")
	flag.StringVar(&opts.IgnoreCodes, "ic", "404", "")
	flag.BoolVar(&opts.Insecure, "insecure", false, "")
	flag.BoolVar(&opts.FollowRedirect, "follow-redirects", false, "")
	flag.StringVar(&opts.UserAgent, "user-agent", "")
	flag.BoolVar(&opts.Silent, "silent", false, "")
	flag.BoolVar(&opts.Verbose, "v", false, "")
	flag.BoolVar(&opts.Verbose, "verbose", false, "")

	help := flag.Bool("h", false, "")
	flag.BoolVar(help, "help", false, "")
	flag.Usage = func() { showHelp() }
	flag.Parse()

	if *help || opts.URL == "" || opts.Wordlist == "" {
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

func worker(ctx context.Context, client *http.Client, opts *Options, jobs <-chan string, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for path := range jobs {
		if opts.Delay > 0 {
			time.Sleep(time.Duration(opts.Delay) * time.Millisecond)
		}

		target, err := url.JoinPath(opts.URL, path)
		if err != nil {
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "[ERR] Bad URL join: %s + %s | %v\n", opts.URL, path, err)
			}
			continue
		}

		var resp *http.Response
		var lastErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				backoff := time.Duration(attempt) * time.Second
				if opts.Verbose {
					fmt.Fprintf(os.Stderr, "[RETRY] %s (attempt %d/%d, backoff %s)\n", target, attempt, maxRetries, backoff)
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
				if attempt < maxRetries {
					continue
				}
			}
			break
		}

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

		if !shouldIgnore(resp.StatusCode, opts.ignoreList) {
			results <- Result{
				URL:        target,
				Method:     opts.Method,
				StatusCode: resp.StatusCode,
				ContentLen: int64(len(body)),
			}
		}
	}
}

func main() {
	opts := parseFlags()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	file, err := os.Open(opts.Wordlist)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[FATAL] Could not open wordlist: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	client := buildClient(opts)

	jobs := make(chan string, opts.Threads*2)
	results := make(chan Result, opts.Threads*2)
	var wg sync.WaitGroup

	for i := 0; i < opts.Threads; i++ {
		wg.Add(1)
		go worker(ctx, client, opts, jobs, results, &wg)
	}

	go func() {
		for r := range results {
			if opts.Silent {
				fmt.Println(r.URL)
				continue
			}

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

			fmt.Printf("[%s%d%s] [%s] %s [Size: %d]\n", color, r.StatusCode, reset, r.Method, r.URL, r.ContentLen)
		}
	}()

	if !opts.Silent {
		fmt.Print(banner)
		fmt.Printf("  %s %s by %s\n", "ep", version, devName)
		fmt.Println("  Advanced API Endpoint Discovery Engine")
		fmt.Println()
		fmt.Printf("[*] Target   : %s\n", opts.URL)
		fmt.Printf("[*] Method   : %s\n", opts.Method)
		fmt.Printf("[*] Wordlist : %s\n", opts.Wordlist)
		if opts.Proxy != "" {
			fmt.Printf("[*] Proxy    : %s\n", opts.Proxy)
		}
		if len(opts.Headers) > 0 {
			fmt.Printf("[*] Headers  : %d custom header(s)\n", len(opts.Headers))
		}
		fmt.Printf("[*] Threads  : %d | Delay: %dms | Timeout: %ds\n", opts.Threads, opts.Delay, opts.Timeout)
		fmt.Printf("[*] Ignore   : %v\n", opts.ignoreList)
		if opts.Insecure {
			fmt.Println("[!] Warning  : TLS verification disabled")
		}
		fmt.Println("--------------------------------------------------")
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			goto done
		default:
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		jobs <- line
	}

done:
	close(jobs)
	wg.Wait()
	close(results)

	time.Sleep(100 * time.Millisecond)

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "[ERR] Reading wordlist: %v\n", err)
	}
}
