import subprocess
import string
import secrets
import os
import sys

def python_runner(code):
    os.makedirs("tmp", exist_ok=True)

    alphabet = string.ascii_letters + string.digits
    random_str = "".join(secrets.choice(alphabet) for _ in range(10))
    file_name = os.path.join("tmp", f"python_{random_str}.py")

    with open(file_name, "w", encoding="utf-8") as f:
        f.write(code)

    try:
        result = subprocess.run(
            [sys.executable, file_name],
            capture_output=True, text=True, timeout=120
        )
        return result.stdout + result.stderr
    except subprocess.TimeoutExpired:
        return "에러: 코드 실행이 30초를 초과해 중단됐습니다"
    finally:
        if os.path.exists(file_name):
            os.remove(file_name)