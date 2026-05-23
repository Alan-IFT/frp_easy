package httpapi

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/frp-easy/frp-easy/internal/downloader"
	"github.com/frp-easy/frp-easy/internal/procmgr"
	"github.com/go-chi/chi/v5"
)

// SystemReady 是 GET /api/v1/system/ready 的响应体。
type SystemReady struct {
	Initialized bool     `json:"initialized"`
	BinMissing  []string `json:"binMissing"`
	Version     string   `json:"version"`
}

func (h *handlers) systemReady(w http.ResponseWriter, r *http.Request) {
	admin, err := h.deps.Store.GetAdmin(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, CodeInternal, "读取管理员失败", "")
		return
	}
	missing := []string{}
	if h.deps.Locator != nil {
		missing = h.deps.Locator.Missing()
	}
	resp := SystemReady{
		Initialized: admin != nil,
		BinMissing:  missing,
		Version:     h.deps.Version,
	}
	writeJSON(w, http.StatusOK, resp)
}

// --- Public IP detection (B-13, B-14, AC-12, AC-13) ---

// ipResult holds the result of one public-IP lookup attempt.
// T-018 §B：新增 Source 字段（胜出源标识，env / ipify / ip.cn 等）。
type ipResult struct {
	IP       string
	Advisory string // non-empty for IPv6 addresses
	ErrMsg   string
	Source   string // optional: 胜出源标识
}

// ipCache is a process-scoped in-memory cache for the public IP result (TTL 5 min).
// The zero value is safe to use.
// C-1 gate: the `ipCache ipCache` field is declared in router.go's handlers struct.
type ipCache struct {
	mu        sync.Mutex
	result    *ipResult
	fetchedAt time.Time
}

// PublicIPResponse is the response body for GET /api/v1/system/public-ip.
// Always HTTP 200 (B-14 requirement — never 4xx/5xx on timeout).
// T-018 §B 扩展：新增可选 Source（胜出源标识，便于运维诊断）。
type PublicIPResponse struct {
	IP       string `json:"ip,omitempty"`
	Error    string `json:"error,omitempty"`
	Advisory string `json:"advisory,omitempty"` // IPv6 usage hint
	Source   string `json:"source,omitempty"`   // 胜出源 (ipify / ip.cn / env ...)
}

// DownloadBinRequest is the request body for POST /api/v1/system/download-bin.
type DownloadBinRequest struct {
	Kind string `json:"kind"` // "frpc" | "frps"
}

// DownloadStatusResponse is an alias for downloader.DownloadState (used for clarity in docs).
type DownloadStatusResponse = downloader.DownloadState

// systemPublicIP handles GET /api/v1/system/public-ip.
// Caches results for 5 minutes to avoid hammering external services (B-14).
func (h *handlers) systemPublicIP(w http.ResponseWriter, r *http.Request) {
	const cacheTTL = 5 * time.Minute

	h.ipCache.mu.Lock()
	if h.ipCache.result != nil && time.Since(h.ipCache.fetchedAt) < cacheTTL {
		result := *h.ipCache.result
		h.ipCache.mu.Unlock()
		h.deps.Logger.Debug("public-ip cache hit", "ip", result.IP)
		respondWithIPResult(w, result)
		return
	}
	h.ipCache.mu.Unlock()

	// Fetch from external services (max 3s total, NF-P1).
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	result := fetchPublicIP(ctx, defaultIPSources)

	// Store in cache (concurrent requests may both miss and both fetch — acceptable for MVP).
	h.ipCache.mu.Lock()
	h.ipCache.result = &result
	h.ipCache.fetchedAt = time.Now()
	h.ipCache.mu.Unlock()

	respondWithIPResult(w, result)
}

// downloadBin handles POST /api/v1/system/download-bin → 202 or 409 (AC-2, AC-4).
func (h *handlers) downloadBin(w http.ResponseWriter, r *http.Request) {
	if h.deps.Downloader == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
		return
	}

	var req DownloadBinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}

	if err := h.deps.Downloader.Start(req.Kind); err != nil {
		switch {
		case errors.Is(err, downloader.ErrAlreadyInProgress):
			writeError(w, http.StatusConflict, CodeProcBusy, "下载已在进行中", "")
		case errors.Is(err, downloader.ErrBadKind):
			writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "kind 必须为 frpc 或 frps", "kind")
		case errors.Is(err, downloader.ErrUnsupportedOS):
			writeError(w, http.StatusServiceUnavailable, CodeInternal, "不支持的操作系统", "")
		default:
			writeError(w, http.StatusInternalServerError, CodeInternal, err.Error(), "")
		}
		return
	}

	writeJSON(w, http.StatusAccepted, map[string]bool{"ok": true})
}

// downloadStatus handles GET /api/v1/system/download-status/{kind} → 200 DownloadState (AC-2).
func (h *handlers) downloadStatus(w http.ResponseWriter, r *http.Request) {
	if h.deps.Downloader == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
		return
	}

	kind := chi.URLParam(r, "kind")
	st, ok := h.deps.Downloader.Status(kind)
	if !ok {
		writeError(w, http.StatusNotFound, CodeNotFound, "无效的 kind，必须是 frpc 或 frps", "")
		return
	}

	writeJSON(w, http.StatusOK, st)
}

// --- internal helpers for public IP detection (T-018 §B 扩展) ---

// ipSource 是一个 IP 探测候选源。
type ipSource struct {
	name    string
	url     string
	parser  func([]byte) (string, error) // JSON / HTML 不同 parser
	maxBody int64                          // body 读取上限（防大页面 OOM）
}

// defaultIPSources 是生产环境默认探测候选清单（02 §B.1，PM-DECIDED）：
// 2 国际源（ipify / my-ip.io）+ 3 大陆友好源（ip.cn / bilibili / ip.cn-HTML 兜底）。
var defaultIPSources = []ipSource{
	// 国际源（沿用）
	{"ipify", "https://api.ipify.org?format=json", parseIPFromIPField, 32 << 10},
	{"my-ip.io", "https://api.my-ip.io/json", parseIPFromIPField, 32 << 10},
	// 大陆友好源
	{"ip.cn", "https://ip.cn/api/index?ip=&type=0", parseIPCnJSON, 32 << 10},
	{"bilibili", "https://api.live.bilibili.com/ip_service/v1/ip_service/get_ip_addr", parseBilibiliJSON, 32 << 10},
	{"ip.cn-html", "https://www.ip.cn/", parseFirstIPv4FromHTML, 256 << 10},
}

// ipv4PublicRE 抽取 HTML 中第一个看起来像 IPv4 的字符串。
var ipv4PublicRE = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`)

// parseIPFromIPField 解析 {"ip":"..."} 形式（ipify / my-ip.io）。
func parseIPFromIPField(data []byte) (string, error) {
	return downloader.ParseIPFromJSON(data)
}

// parseIPCnJSON 解析 ip.cn 的响应：
// 顶层可能是 `{"ip":"..."}`，也可能是 `{"code":0,"data":{"ip":"..."}}`。
func parseIPCnJSON(data []byte) (string, error) {
	var r struct {
		IP   string `json:"ip"`
		Data struct {
			IP string `json:"ip"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	if r.IP != "" {
		return r.IP, nil
	}
	if r.Data.IP != "" {
		return r.Data.IP, nil
	}
	return "", errors.New("ip.cn 响应缺少 ip 字段")
}

// parseBilibiliJSON 解析 bilibili IP service：`{"data":{"addr":"..."}}`。
func parseBilibiliJSON(data []byte) (string, error) {
	var r struct {
		Data struct {
			Addr string `json:"addr"`
		} `json:"data"`
	}
	if err := json.Unmarshal(data, &r); err != nil {
		return "", err
	}
	if r.Data.Addr == "" {
		return "", errors.New("bilibili 响应缺少 data.addr")
	}
	return r.Data.Addr, nil
}

// parseFirstIPv4FromHTML 从 HTML 文本中抽取首个**合法公网** IPv4。
// 私有 / 回环 / 链路本地段会被跳过，防止页面里嵌广告 IP 污染（R-9）。
func parseFirstIPv4FromHTML(data []byte) (string, error) {
	for _, m := range ipv4PublicRE.FindAll(data, -1) {
		ip := net.ParseIP(string(m))
		if ip == nil || ip.To4() == nil {
			continue
		}
		if ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() {
			continue
		}
		return string(m), nil
	}
	return "", errors.New("HTML 中未提取到合法公网 IPv4")
}

// fetchPublicIP 并发探测全部候选源，首个返回合法 IP 的胜出；其它请求随 ctx.cancel 取消。
// 总预算由调用方传入的 ctx 控制（默认 3s）。
//
// FR-B.6（T-018 B-1）：本任务**首次**在 Go 后端引入 `FRP_EASY_PUBLIC_IP` 环境变量短路。
// 此前该 env 只在 install.sh / install.ps1 安装期被读取；本任务把它扩展到运行期。
// 优先级最高 —— 命中则不发任何 HTTP，避免国内 VM 上 ipify 等源全失败时仍能给出 IP。
//
// sources 参数化（B 测试 seam）：handler 调用时传 defaultIPSources，单测可注入 mock httptest。
func fetchPublicIP(ctx context.Context, sources []ipSource) ipResult {
	// env 短路（T-018 B-1）
	if v := strings.TrimSpace(os.Getenv("FRP_EASY_PUBLIC_IP")); v != "" {
		r := buildIPResult(v)
		r.Source = "env"
		return r
	}

	if len(sources) == 0 {
		return ipResult{ErrMsg: "检测超时，请手动查询"}
	}

	// 并发探：任一成功立即返回，cancel 其它在飞请求。
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	type winnerMsg struct {
		ip, source string
	}
	ch := make(chan winnerMsg, len(sources))
	var wg sync.WaitGroup
	for _, src := range sources {
		wg.Add(1)
		go func(s ipSource) {
			defer wg.Done()
			ip, err := fetchIPFromSource(subCtx, s)
			if err == nil && ip != "" {
				select {
				case ch <- winnerMsg{ip, s.name}:
				default:
				}
			}
		}(src)
	}
	// 关闭通道等所有 goroutine 完成（成功或被 cancel）。
	doneAll := make(chan struct{})
	go func() {
		wg.Wait()
		close(doneAll)
	}()

	select {
	case w, ok := <-ch:
		if ok {
			cancel() // 取消其它在飞请求
			r := buildIPResult(w.ip)
			r.Source = w.source
			return r
		}
	case <-subCtx.Done():
	case <-doneAll:
	}
	return ipResult{ErrMsg: "检测超时，请手动查询"}
}

// fetchIPFromSource：单源 GET + 限流 body + parser + IP 校验。
// 所有出站请求带 `User-Agent: frp_easy`（T-014 insight L37 / FR-B.8）。
func fetchIPFromSource(ctx context.Context, s ipSource) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.url, nil)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "frp_easy")
	req.Header.Set("Accept", "application/json,text/html")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, s.maxBody))
	if err != nil {
		return "", fmt.Errorf("read body: %w", err)
	}
	ip, err := s.parser(data)
	if err != nil {
		return "", err
	}
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("source %s 返回非合法 IP: %q", s.name, ip)
	}
	return ip, nil
}

// buildIPResult constructs an ipResult and adds an IPv6 advisory if applicable.
// 注意：env 短路场景下用户可能注入任意值（含非合法 IP，B-B.8 [PM-DECIDED]），仍透传。
func buildIPResult(ip string) ipResult {
	r := ipResult{IP: ip}
	if parsed := net.ParseIP(ip); parsed != nil && parsed.To4() == nil {
		r.Advisory = fmt.Sprintf("IPv6 地址，frpc serverAddr 填写时请加方括号 [%s]", ip)
	}
	return r
}

// respondWithIPResult writes a PublicIPResponse (always HTTP 200, B-14).
func respondWithIPResult(w http.ResponseWriter, result ipResult) {
	resp := PublicIPResponse{}
	if result.ErrMsg != "" {
		resp.Error = result.ErrMsg
	} else {
		resp.IP = result.IP
		resp.Advisory = result.Advisory
		resp.Source = result.Source
	}
	writeJSON(w, http.StatusOK, resp)
}

// health handles GET /api/v1/health — 轻量存活检查，不经过 ReadyGate。
func (h *handlers) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": h.deps.Version,
	})
}

// --- T-018 A: 二进制上传入口 ---

// UploadBinResponse 是 POST /api/v1/system/upload-bin 的成功响应（200）。
type UploadBinResponse struct {
	Ok       bool   `json:"ok"`
	Kind     string `json:"kind"`
	SHA256   string `json:"sha256"`
	Size     int64  `json:"size"`
	Path     string `json:"path"`               // 相对 root 的相对路径（NF-S 既有口径）
	Advisory string `json:"advisory,omitempty"` // 仅当同 kind 子进程正在运行时附带
}

// uploadBinMaxBytes 是单次上传 binary 的大小上限（PM-DECIDED 64 MiB，FR-A.3 a）。
// FRP 1.x 单 binary 实测 ~20 MiB，留 3× 安全余量。
const uploadBinMaxBytes int64 = 64 << 20

// uploadLockFrpc / uploadLockFrps：按 kind 拆 2 把锁（B-A.7）。
// 同 kind 并发上传 → 后到者 409 PROC_BUSY。
var (
	uploadLockFrpc sync.Mutex
	uploadLockFrps sync.Mutex
)

func pickUploadLock(kind string) *sync.Mutex {
	if kind == "frpc" {
		return &uploadLockFrpc
	}
	return &uploadLockFrps
}

// uploadBin handles POST /api/v1/system/upload-bin（T-018 A 模块）。
//
// 设计要点（02 §A.2 v2 修订）：
//   - http.MaxBytesReader 在 ParseMultipartForm **之前**裹一层，防 OOM；超大直接 413。
//   - ParseMultipartForm(8 MiB) → FormValue / FormFile，不依赖客户端字段顺序（B-6 修订）。
//   - 文件头校验仅 MZ / ELF + 平台一致性（B-11 修订）。
//   - 落盘走 downloader.Install（共享下载链路的原子 rename + chmod + Windows fallback）。
//   - 上传期间若同 kind 下载在跑 → 409 PROC_BUSY；自身锁守护同 kind 并发上传。
func (h *handlers) uploadBin(w http.ResponseWriter, r *http.Request) {
	if h.deps.Downloader == nil {
		writeError(w, http.StatusServiceUnavailable, CodeInternal, "下载器未初始化", "")
		return
	}

	const maxBodyBytes = uploadBinMaxBytes + (1 << 20) // +1 MiB 容 multipart 包头
	const parseMemory = int64(8 << 20)                 // 8 MiB 走内存，剩余 spill 到 tmp

	// 1. MaxBytesReader 防 OOM —— 必须在 ParseMultipartForm 之前裹一层。
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)

	// 2. 解析 multipart（顺序无关，自动磁盘 spill；不依赖客户端 append 顺序）。
	if err := r.ParseMultipartForm(parseMemory); err != nil {
		// MaxBytesReader 触发 *http.MaxBytesError；与"非 multipart"区分（B-6）。
		var mb *http.MaxBytesError
		if errors.As(err, &mb) {
			writeError(w, http.StatusRequestEntityTooLarge, CodeValidationFailed, "文件超过 64 MiB 上限", "file")
			return
		}
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求不是合法的 multipart/form-data", "")
		return
	}

	// 3. 字段白名单：仅读 kind / file（NF-A.2）。
	kind := strings.TrimSpace(r.FormValue("kind"))
	if kind == "" {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "缺少字段：kind", "kind")
		return
	}
	if kind != "frpc" && kind != "frps" {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "kind 必须为 frpc 或 frps", "kind")
		return
	}
	file, fh, ferr := r.FormFile("file")
	if ferr != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "缺少字段：file", "file")
		return
	}
	defer file.Close()
	if fh.Size == 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "上传文件为空", "file")
		return
	}
	if fh.Size > uploadBinMaxBytes {
		writeError(w, http.StatusRequestEntityTooLarge, CodeValidationFailed, "文件超过 64 MiB 上限", "file")
		return
	}

	// 4. 同 kind 并发上传 / 与下载互斥。
	lk := pickUploadLock(kind)
	if !lk.TryLock() {
		writeError(w, http.StatusConflict, CodeProcBusy, "上传进行中，请稍后重试", "")
		return
	}
	defer lk.Unlock()
	if st, ok := h.deps.Downloader.Status(kind); ok && st.Status == downloader.StatusDownloading {
		writeError(w, http.StatusConflict, CodeProcBusy, "下载进行中，请稍后再上传或取消下载", "")
		return
	}

	// 5. 文件头校验（前 64 字节 peek，不消费流；B-11 修订仅 MZ / ELF）。
	br := bufio.NewReaderSize(file, 4096)
	head, _ := br.Peek(64)
	if len(head) == 0 {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, "上传文件为空", "file")
		return
	}
	if err := validateBinaryHeader(head, runtime.GOOS); err != nil {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed, err.Error(), "file")
		return
	}

	// 6. 落盘走共享 Install。
	startTime := time.Now()
	sha, n, finalPath, err := h.deps.Downloader.Install(kind, br, uploadBinMaxBytes)
	if errors.Is(err, downloader.ErrFileTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, CodeValidationFailed, "文件超过 64 MiB 上限", "file")
		return
	}
	if err != nil {
		// errno 透传给用户（Windows 文件被锁等场景，B-A.12 / R-7）。
		writeError(w, http.StatusInternalServerError, CodeInternal, "落盘失败: "+err.Error(), "")
		return
	}

	// 7. 决定是否附 advisory（同 kind 子进程正在运行 → 提示用户手动重启）。
	advisory := ""
	if h.deps.ProcMgr != nil {
		info := h.deps.ProcMgr.Status(kind)
		if info.State == procmgr.StateRunning {
			advisory = "上传成功；如需立即生效请到运行控制重启 " + kind
		}
	}

	// 8. 日志 + 响应。
	if h.deps.Logger != nil {
		h.deps.Logger.Info("upload-bin success",
			"kind", kind, "size", n, "sha256", sha,
			"elapsed_ms", time.Since(startTime).Milliseconds())
	}

	relPath := finalPath
	if h.deps.Locator != nil {
		if rel, rerr := filepath.Rel(h.deps.Locator.Root(), finalPath); rerr == nil {
			relPath = filepath.ToSlash(rel)
		}
	}

	writeJSON(w, http.StatusOK, UploadBinResponse{
		Ok:       true,
		Kind:     kind,
		SHA256:   sha,
		Size:     n,
		Path:     relPath,
		Advisory: advisory,
	})
}

// validateBinaryHeader 校验 head 前导字节是否与运行平台一致（FR-A.4 / B-11 修订）。
//
// 设计选择（B-11）：仅 MZ 即接受 PE；不做 offset 0x3C 处 PE\0\0 二次校验。
// 理由：peek 64 字节不足以保证读到 PE\0\0；落盘后若 binary 非真正可执行，
// 由 procmgr 启动失败时的 lastErr 暴露给前端（错误已有显示通道）。
func validateBinaryHeader(head []byte, goos string) error {
	if len(head) < 4 {
		return errors.New("不是合法的二进制文件（文件过短）")
	}
	isELF := head[0] == 0x7F && head[1] == 'E' && head[2] == 'L' && head[3] == 'F'
	isPE := len(head) >= 2 && head[0] == 'M' && head[1] == 'Z'
	switch goos {
	case "linux":
		if !isELF {
			if isPE {
				return errors.New("上传的二进制平台不匹配（本机=linux，文件=windows）")
			}
			return errors.New("不是合法的二进制文件（缺少 ELF 文件头）")
		}
	case "windows":
		if !isPE {
			if isELF {
				return errors.New("上传的二进制平台不匹配（本机=windows，文件=linux）")
			}
			return errors.New("不是合法的二进制文件（缺少 PE 文件头）")
		}
	default:
		return errors.New("不支持的操作系统")
	}
	return nil
}

// --- T-018 C-3: 端口可用性探测 ---

// PortProbeRequest 是 POST /api/v1/system/probe-ports 的请求体。
type PortProbeRequest struct {
	Ports []int `json:"ports"`
}

// PortProbeResult 是单个端口的探测结果。
type PortProbeResult struct {
	Port      int    `json:"port"`
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// PortProbeResponse 是 probePorts 的成功响应。
type PortProbeResponse struct {
	Results []PortProbeResult `json:"results"`
}

const (
	portProbeMaxCount = 64 // T-018 FR-C.3.4 / AC-C.3.5 上限 64（探测轻量，可大于批量 cap 32）
	portProbeTimeout  = 200 * time.Millisecond
)

// probePorts handles POST /api/v1/system/probe-ports（T-018 C-3）。
//
// 安全（FR-C.3.7 / R-8）：硬编码本机 wildcard 监听，**不接受** host 参数，
// 防止被滥用做端口扫描其它主机。
//
// 实现：
//   - ports 长度 ≤ portProbeMaxCount，否则 422。
//   - 端口 ∈ [1, 65535]，单个非法 → 422。
//   - 端口 < 1024（特权端口）→ 直接返回 available=false reason="privileged"，**不实际探**
//     （FR-C.3.3，避免在 root 启动下伪绑定误判）。
//   - 1024-65535 → net.Listen("tcp", ":N") （B-9：dual-stack wildcard，IPv4+IPv6）+ 立即 Close。
//   - 单端口 200ms timeout；总 5s timeout。
//   - 去重：ports 含重复时去重后探测。
func (h *handlers) probePorts(w http.ResponseWriter, r *http.Request) {
	var req PortProbeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, CodeValidationFailed, "请求体不是合法 JSON", "")
		return
	}

	// 去重（保持原顺序）
	seen := make(map[int]bool, len(req.Ports))
	ports := make([]int, 0, len(req.Ports))
	for _, p := range req.Ports {
		if !seen[p] {
			seen[p] = true
			ports = append(ports, p)
		}
	}

	if len(ports) > portProbeMaxCount {
		writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
			fmt.Sprintf("单次最多探测 %d 个端口", portProbeMaxCount), "ports")
		return
	}
	for _, p := range ports {
		if p < 1 || p > 65535 {
			writeError(w, http.StatusUnprocessableEntity, CodeValidationFailed,
				"端口必须在 1-65535 之间", "ports")
			return
		}
	}

	// 总 5s timeout 由 r.Context 派生；每端口 200ms 由 probeOnePort 内部控制。
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	results := make([]PortProbeResult, len(ports))
	var wg sync.WaitGroup
	for i, p := range ports {
		wg.Add(1)
		go func(idx, port int) {
			defer wg.Done()
			results[idx] = probeOnePort(ctx, port)
		}(i, p)
	}
	wg.Wait()

	writeJSON(w, http.StatusOK, PortProbeResponse{Results: results})
}

// probeOnePort 探测单端口。
// 特权端口（<1024）不实际探测，直接返回提示。
// 普通端口走 net.Listen("tcp", ":N") + 立即 Close（B-9 dual-stack wildcard）。
func probeOnePort(ctx context.Context, port int) PortProbeResult {
	if port < 1024 {
		return PortProbeResult{Port: port, Available: false, Reason: "privileged"}
	}

	// 200ms timeout —— 用 ListenConfig.Listen 取得 ctx 取消能力。
	subCtx, cancel := context.WithTimeout(ctx, portProbeTimeout)
	defer cancel()

	var lc net.ListenConfig
	ln, err := lc.Listen(subCtx, "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return PortProbeResult{Port: port, Available: false, Reason: "in_use"}
	}
	_ = ln.Close()
	return PortProbeResult{Port: port, Available: true}
}
