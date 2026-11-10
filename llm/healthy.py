from openai import OpenAI
from config import BASE_URL, MODEL_NAME, API_KEY

def llm_health_check():
    client = OpenAI(
        base_url=BASE_URL + "/v1",
        api_key=API_KEY
    )

    try:
        res = client.chat.completions.create(
            model=MODEL_NAME,
            messages=[{"role": "user", "content": "ping"}],
            max_tokens=5
        )
        return "pong"
    except Exception as e:
        return f"[ERROR] {e}"