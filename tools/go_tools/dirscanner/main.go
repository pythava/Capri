package main

import (
	"bufio"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	defaultConcurrency = 50
	maxConcurrency     = 500
	defaultTimeoutSec  = 5
	maxBodyRead        = 1 << 20 // 응답 바디는 최대 1MB까지만 읽어서 해시/크기 계산
	defaultUserAgent   = "go-dirscanner/1.0"
)

var defaultStatusCodes = []int{200, 204, 301, 302, 307, 308, 401, 403, 405}

// ---------- 요청 / 응답 구조체 ----------

type ScanRequest struct {
	TargetURL     string            `json:"target_url"`
	Words         []string          `json:"words"`         // 직접 전달하는 단어 목록
	WordlistPath  string            `json:"wordlist_path"` // 대용량 워드리스트용 파일 경로
	Extensions    []string          `json:"extensions"`    // ["php","html"] (점 없이)
	StatusCodes   []int             `json:"status_codes"`  // 수집할 상태 코드 (기본값 있음)
	Concurrency   int               `json:"concurrency"`
	TimeoutSec    int               `json:"timeout_sec"`
	UserAgent     string            `json:"user_agent"`
	Headers       map[string]string `json:"headers"`
	SkipTLSVerify bool              `json:"skip_tls_verify"`
	AddSlash      bool              `json:"add_slash"` // word/ 도 같이 요청

	// 와일드카드 / 필터링
	NoWildcardFilter bool    `json:"no_wildcard_filter"` // true면 와일드카드 필터 끔
	SizeTolerance    int64   `json:"size_tolerance"`     // 응답 크기 허용 오차 (바이트)
	ExcludeLengths   []int64 `json:"exclude_lengths"`    // 이 크기의 응답은 버림
}

type ScanResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
	Size       int64  `json:"size"`
	Redirect   string `json:"redirect,omitempty"`
	Truncated  bool   `json:"truncated,omitempty"` // 응답이 maxBodyRead를 넘어서 크기가 부정확할 수 있음
}

type WildcardSig struct {
	StatusCode int    `json:"status_code"`
	Size       int64  `json:"size"`
	Redirect   string `json:"redirect,omitempty"`
}

type ScanResponse struct {
	Target           string        `json:"target"`
	WildcardDetected bool          `json:"wildcard_detected"`
	WildcardInfo     []WildcardSig `json:"wildcard_info,omitempty"`
	TotalRequests    int64         `json:"total_requests"`
	Errors           int64         `json:"errors"`
	LastError        string        `json:"last_error,omitempty"`
	Filtered         int64         `json:"filtered"` // 와일드카드로 판단되어 제외된 수
	DurationMs       int64         `json:"duration_ms"`
	Results          []ScanResult  `json:"results"`
}

// ---------- 한 번의 요청 결과 ----------

type Probe struct {
	Status    int
	Size      int64
	Hash      string
	Location  string // 원본 Location 헤더
	NormLoc   string // 경로를 {W}로 치환한 Location (와일드카드 비교용)
	Truncated bool
}

func isRedirect(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

func fetch(client *http.Client, req *ScanRequest, target, path string) (Probe, error) {
	httpReq, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return Probe{}, err
	}
	ua := req.UserAgent
	if ua == "" {
		ua = defaultUserAgent
	}
	httpReq.Header.Set("User-Agent", ua)
	for k, v := range req.Headers {
		httpReq.Header.Set(k, v)
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		return Probe{}, err
	}
	defer resp.Body.Close()

	h := sha256.New()
	n, _ := io.Copy(h, io.LimitReader(resp.Body, maxBodyRead))
	truncated := n >= maxBodyRead
	// 남은 바디도 (제한적으로) 비워서 커넥션 재사용 확률을 높임
	io.Copy(io.Discard, io.LimitReader(resp.Body, maxBodyRead))

	size := n
	if resp.ContentLength > size {
		size = resp.ContentLength // Content-Length 헤더가 있으면 그게 더 정확
	}

	loc := resp.Header.Get("Location")
	norm := ""
	if loc != "" && path != "" {
		norm = strings.ReplaceAll(loc, path, "{W}")
	}

	return Probe{
		Status:    resp.StatusCode,
		Size:      size,
		Hash:      hex.EncodeToString(h.Sum(nil)),
		Location:  loc,
		NormLoc:   norm,
		Truncated: truncated,
	}, nil
}

// ---------- 와일드카드 감지 ----------

type wildcardFilter struct {
	sigs      []Probe
	tolerance int64
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func (f *wildcardFilter) matches(p Probe) bool {
	if f == nil {
		return false
	}
	for _, s := range f.sigs {
		if s.Status != p.Status {
			continue
		}
		if isRedirect(p.Status) {
			if s.NormLoc != "" && s.NormLoc == p.NormLoc {
				return true
			}
			continue
		}
		if s.Hash == p.Hash {
			return true
		}
		if abs64(s.Size-p.Size) <= f.tolerance {
			return true
		}
	}
	return false
}

func randString(n int) string {
	b := make([]byte, (n+1)/2)
	rand.Read(b)
	return hex.EncodeToString(b)[:n]
}

func detectWildcard(client *http.Client, req *ScanRequest, base string, allowed map[int]bool) *wildcardFilter {
	var sigs []Probe

	for _, length := range []int{8, 16, 24} {
		name := randString(length)
		paths := buildPaths(name, req.Extensions, req.AddSlash)
		for _, p := range paths {
			probe, err := fetch(client, req, base+"/"+p, p)
			if err != nil {
				continue
			}
			if allowed[probe.Status] {
				sigs = append(sigs, probe)
			}
		}
	}
	if len(sigs) == 0 {
		return nil
	}

	var minSize, maxSize int64 = -1, -1
	for _, s := range sigs {
		if isRedirect(s.Status) {
			continue
		}
		if minSize == -1 || s.Size < minSize {
			minSize = s.Size
		}
		if s.Size > maxSize {
			maxSize = s.Size
		}
	}
	tol := req.SizeTolerance
	if minSize != -1 && maxSize-minSize > tol {
		tol = maxSize - minSize
	}
	return &wildcardFilter{sigs: sigs, tolerance: tol}
}

// ---------- 워드리스트 ----------

func buildPaths(word string, exts []string, addSlash bool) []string {
	word = strings.TrimLeft(strings.TrimSpace(word), "/")
	if word == "" {
		return nil
	}
	paths := []string{word}
	if addSlash && !strings.HasSuffix(word, "/") {
		paths = append(paths, word+"/")
	}
	for _, ext := range exts {
		ext = strings.TrimLeft(ext, ".")
		if ext != "" {
			paths = append(paths, word+"."+ext)
		}
	}
	return paths
}

func loadWords(req *ScanRequest) ([]string, error) {
	words := append([]string(nil), req.Words...)
	if req.WordlistPath != "" {
		f, err := os.Open(req.WordlistPath)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			words = append(words, line)
		}
		if err := sc.Err(); err != nil {
			return nil, err
		}
	}
	return words, nil
}

// ---------- 핵심 스캔 로직 (HTTP 핸들러에서 분리) ----------

func runScan(req *ScanRequest) (ScanResponse, error) {
	if req.Concurrency <= 0 {
		req.Concurrency = defaultConcurrency
	}
	if req.Concurrency > maxConcurrency {
		req.Concurrency = maxConcurrency
	}
	if req.TimeoutSec <= 0 {
		req.TimeoutSec = defaultTimeoutSec
	}
	if len(req.StatusCodes) == 0 {
		req.StatusCodes = defaultStatusCodes
	}

	words, err := loadWords(req)
	if err != nil {
		return ScanResponse{}, fmt.Errorf("wordlist error: %w", err)
	}
	if len(words) == 0 {
		return ScanResponse{}, fmt.Errorf("words 또는 wordlist_path 가 필요합니다")
	}

	allowed := make(map[int]bool, len(req.StatusCodes))
	for _, c := range req.StatusCodes {
		allowed[c] = true
	}
	excluded := make(map[int64]bool, len(req.ExcludeLengths))
	for _, l := range req.ExcludeLengths {
		excluded[l] = true
	}

	transport := &http.Transport{
		MaxIdleConns:        req.Concurrency,
		MaxIdleConnsPerHost: req.Concurrency,
		IdleConnTimeout:     30 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: req.SkipTLSVerify},
	}
	defer transport.CloseIdleConnections()

	client := &http.Client{
		Timeout:   time.Duration(req.TimeoutSec) * time.Second,
		Transport: transport,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	start := time.Now()
	base := strings.TrimRight(req.TargetURL, "/")

	var filter *wildcardFilter
	if !req.NoWildcardFilter {
		filter = detectWildcard(client, req, base, allowed)
	}

	jobs := make(chan string, req.Concurrency*2)
	go func() {
		defer close(jobs)
		for _, word := range words {
			for _, p := range buildPaths(word, req.Extensions, req.AddSlash) {
				jobs <- p
			}
		}
	}()

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		results  = []ScanResult{}
		lastErr  string
		total    int64
		errCount int64
		filtered int64
	)

	for i := 0; i < req.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				target := base + "/" + path
				p, err := fetch(client, req, target, path)
				atomic.AddInt64(&total, 1)
				if err != nil {
					atomic.AddInt64(&errCount, 1)
					mu.Lock()
					lastErr = err.Error()
					mu.Unlock()
					continue
				}
				if !allowed[p.Status] || excluded[p.Size] {
					continue
				}
				if filter.matches(p) {
					atomic.AddInt64(&filtered, 1)
					continue
				}
				mu.Lock()
				results = append(results, ScanResult{
					URL:        target,
					StatusCode: p.Status,
					Size:       p.Size,
					Redirect:   p.Location,
					Truncated:  p.Truncated,
				})
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	sort.Slice(results, func(i, j int) bool { return results[i].URL < results[j].URL })

	resp := ScanResponse{
		Target:        req.TargetURL,
		TotalRequests: total,
		Errors:        errCount,
		LastError:     lastErr,
		Filtered:      filtered,
		DurationMs:    time.Since(start).Milliseconds(),
		Results:       results,
	}
	if filter != nil {
		resp.WildcardDetected = true
		for _, s := range filter.sigs {
			resp.WildcardInfo = append(resp.WildcardInfo, WildcardSig{
				StatusCode: s.Status, Size: s.Size, Redirect: s.Location,
			})
		}
	}

	return resp, nil
}

// ---------- CLI 엔트리포인트 ----------

func parseIntList(s string) []int {
	if s == "" {
		return nil
	}
	var out []int
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if v, err := strconv.Atoi(part); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseInt64List(s string) []int64 {
	if s == "" {
		return nil
	}
	var out []int64
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if v, err := strconv.ParseInt(part, 10, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parseStrList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func main() {
	url := flag.String("url", "", "대상 URL (필수)")
	wordlist := flag.String("wordlist", "", "워드리스트 파일 경로 (필수, words와 둘 중 하나는 있어야 함)")
	words := flag.String("words", "", "쉼표로 구분된 단어 목록 (예: admin,login,test)")
	extensions := flag.String("extensions", "", "예: php,html,txt")
	statusCodes := flag.String("status", "", "예: 200,301,403")
	excludeLengths := flag.String("exclude-length", "", "예: 1234,5678")
	concurrency := flag.Int("concurrency", defaultConcurrency, "동시 요청 수 (최대 500)")
	timeoutSec := flag.Int("timeout", defaultTimeoutSec, "요청당 타임아웃(초)")
	userAgent := flag.String("user-agent", "", "커스텀 User-Agent")
	noWildcardFilter := flag.Bool("no-wildcard-filter", false, "와일드카드 필터 끄기")
	addSlash := flag.Bool("add-slash", false, "word/ 형태도 같이 요청")
	skipTLSVerify := flag.Bool("insecure", false, "TLS 인증서 검증 건너뛰기")
	sizeTolerance := flag.Int64("size-tolerance", 0, "응답 크기 허용 오차(바이트)")
	flag.Parse()

	if *url == "" {
		fmt.Fprintln(os.Stderr, "에러: --url은 필수입니다")
		os.Exit(1)
	}
	if *wordlist == "" && *words == "" {
		fmt.Fprintln(os.Stderr, "에러: --wordlist 또는 --words 중 하나는 필수입니다")
		os.Exit(1)
	}

	req := &ScanRequest{
		TargetURL:        *url,
		WordlistPath:     *wordlist,
		Words:            parseStrList(*words),
		Extensions:       parseStrList(*extensions),
		StatusCodes:      parseIntList(*statusCodes),
		ExcludeLengths:   parseInt64List(*excludeLengths),
		Concurrency:      *concurrency,
		TimeoutSec:       *timeoutSec,
		UserAgent:        *userAgent,
		NoWildcardFilter: *noWildcardFilter,
		AddSlash:         *addSlash,
		SkipTLSVerify:    *skipTLSVerify,
		SizeTolerance:    *sizeTolerance,
	}

	resp, err := runScan(req)
	if err != nil {
		// 에러도 JSON으로 출력해서 Python 쪽이 일관되게 파싱할 수 있게 함
		errOut, _ := json.Marshal(map[string]string{"error": err.Error()})
		fmt.Println(string(errOut))
		os.Exit(1)
	}

	out, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintln(os.Stderr, "에러: 결과 직렬화 실패: "+err.Error())
		os.Exit(1)
	}
	fmt.Println(string(out))
}
