import requests
from config import BASE_URL, MODEL_NAME, API_KEY
import json

def req_to_llm(messages, slot_status):

    messages.append({"role": "assistant", "content": f"현재 슬롯 상태 : {slot_status}"})

    res = requests.post(
        f"{BASE_URL}/api/chat",
        json={
            "model": MODEL_NAME,
            "messages": messages,
            "format": "json",   # 여기서 JSON 강제
            "stream": False,
            "options": {"temperature": 0.7}
        }
    )
    res.raise_for_status()

    outer = res.json()
    content_str = outer["message"]["content"]

    return parse_model_json(content_str)

def parse_model_json(content_str):
    try:
        return json.loads(content_str)
    except json.JSONDecodeError:
        import re
        match = re.search(r"\{.*\}", content_str, re.DOTALL)
        if match:
            try:
                return json.loads(match.group())
            except json.JSONDecodeError:
                pass
        raise ValueError(f"JSON 파싱 실패 : {content_str!r}")

    