import subprocess
import json
import os
import platform

def _get_go_binary():
    system = platform.system()
    if system == "Windows":
        name = "dirscanner_windows.exe"
    elif system == "Linux":
        name = "dirscanner_linux"
    elif system == "Darwin":
        name = "dirscanner_mac"
    else:
        raise RuntimeError(f"지원하지 않는 OS: {system}")
    return os.path.join("bin", name)

GO_BINARY = _get_go_binary()


def dir_scan(
    url: str,
    wordlist: str = "",
    words: str = "",
    extensions: str = "",
    status_codes: str = "",
    exclude_length: str = "",
    concurrency: int = 50,
    no_wildcard_filter: bool = False,
    add_slash: bool = False,
    insecure: bool = False,
) -> str:
    """
    Go 디렉토리 스캐너 바이너리를 subprocess로 실행하고 결과를 사람이 읽기 좋은 문자열로 반환한다.
    """
    if not os.path.exists(GO_BINARY):
        return f"에러: Go 바이너리를 찾을 수 없습니다 ({GO_BINARY}). 먼저 빌드했는지 확인하세요."

    if not wordlist and not words:
        return "에러: wordlist 또는 words 중 하나는 반드시 지정해야 합니다."

    cmd = [GO_BINARY, f"--url={url}", f"--concurrency={concurrency}"]

    if wordlist:
        cmd.append(f"--wordlist={wordlist}")
    if words:
        cmd.append(f"--words={words}")
    if extensions:
        cmd.append(f"--extensions={extensions}")
    if status_codes:
        cmd.append(f"--status={status_codes}")
    if exclude_length:
        cmd.append(f"--exclude-length={exclude_length}")
    if no_wildcard_filter:
        cmd.append("--no-wildcard-filter")
    if add_slash:
        cmd.append("--add-slash")
    if insecure:
        cmd.append("--insecure")

    try:
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            timeout=300,
        )
    except subprocess.TimeoutExpired:
        return "에러: 스캔이 300초를 초과해 중단됐습니다. 워드리스트를 줄이거나 concurrency를 높여보세요."
    except FileNotFoundError:
        return f"에러: Go 바이너리를 실행할 수 없습니다 ({GO_BINARY})"

    if not result.stdout.strip():
        return f"에러: 스캐너가 출력을 반환하지 않았습니다 (returncode={result.returncode})\nstderr: {result.stderr.strip()}"

    try:
        data = json.loads(result.stdout.strip())
    except json.JSONDecodeError:
        return f"에러: 스캐너 출력이 JSON이 아닙니다\n{result.stdout[:500]}"

    if "error" in data:
        return f"에러: {data['error']}"

    return format_report(data)


def format_report(data: dict) -> str:
    lines = [
        f"Target   : {data.get('target', 'unknown')}",
        f"Requests : {data.get('total_requests', 0)}  "
        f"(errors: {data.get('errors', 0)}, filtered: {data.get('filtered', 0)}, "
        f"{data.get('duration_ms', 0)} ms)",
    ]

    if data.get("last_error") and data.get("errors", 0) > 0:
        lines.append(f"Last err : {data['last_error']}")

    if data.get("wildcard_detected"):
        lines.append("\n[!] 와일드카드 응답 감지됨 -> 동일 응답은 자동 제외했습니다")
        seen = set()
        for sig in data.get("wildcard_info", []):
            key = (sig.get("status_code"), sig.get("size"), sig.get("redirect", ""))
            if key not in seen:
                seen.add(key)
                lines.append(
                    f"    status={sig.get('status_code')} size={sig.get('size')} {sig.get('redirect', '')}"
                )

    results = data.get("results", [])
    lines.append(f"\nFound {len(results)} paths")
    for r in results:
        extra = f" -> {r['redirect']}" if r.get("redirect") else ""
        truncated_note = " [응답 1MB 초과, 크기 부정확할 수 있음]" if r.get("truncated") else ""
        lines.append(
            f"  [{r['status_code']}] {r['url']}  (size: {r['size']}){extra}{truncated_note}"
        )

    return "\n".join(lines)


if __name__ == "__main__":
    print(dir_scan(url="http://127.0.0.1:8000", words="admin,login,test,config"))