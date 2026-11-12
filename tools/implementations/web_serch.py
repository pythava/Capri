from ddgs import DDGS

def web_search(query, max_results="3"):
    results = []

    with DDGS() as ddgs:
        for r in ddgs.text(query, max_results=int(max_results)):
            results.append(
                {
                    "title": r.get("title"),
                    "href": r.get("href"),
                    "body": r.get("body",)
                }
            )

    return results