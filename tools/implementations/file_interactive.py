# tools/implementations/file_generate.py
import os

def file_generate(path: str, content: str, overwrite: bool = False) -> str:
    if os.path.exists(path) and not overwrite:
        return f"에러: '{path}' 파일이 이미 존재합니다. overwrite=true로 호출하세요."
    
    try:
        dir_name = os.path.dirname(path)
        if dir_name:
            os.makedirs(dir_name, exist_ok=True)
        with open(path, "w", encoding="utf-8") as f:
            f.write(content)
        return f"파일 생성 완료: {path}"
    except Exception as e:
        return f"에러: 파일 생성 실패 - {e}"