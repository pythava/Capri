from tools.list import TOOLS
from tools.implementations.bash import bash

tool_list = TOOLS

def main(llm_action, llm_input, llm_message):
    excute_tool_result = execute_tool(llm_action, llm_input)
    #print(excute_tool_result)
    return excute_tool_result

TOOL_MAP = {
    "bash": bash
}

def execute_tool(llm_action, llm_input):
    print(llm_action)
    print(llm_input)
    
    if llm_action not in TOOLS:
        return f"에러! : unknown tool '{llm_action}'"
    func = TOOL_MAP[llm_action]
    try:
        return func(**llm_input)
    except TypeError as e:
        return f"에러: invalid arguments for '{llm_action}': {e}"
    except Exception as e:
        return f"에러: tool '{llm_action}' failed: {e}"
