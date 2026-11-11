from openai import OpenAI
from llm import client, healthy
from tools import base
from agent.slot import SlotManager
import json

with open("llm/prompts/system.txt", "r", encoding="utf-8") as f:
    system_prompt = f.read()

messages = [
    {"role": "system", "content": system_prompt},
]

def main():

    print(f"[*] LLM의 접속 상태를 확인하는중...")
    result_healthy_check = healthy.llm_health_check()

    if result_healthy_check == "pong":
        manager = SlotManager(max_slots=6)

        print(f"[+] LLM이 정상적으로 연결되어 있습니다")
        while True:
            print("Capri > ", end="")
            user_input_prompt = input()
            messages.append({"role": "user", "content": user_input_prompt})

            while True:
                response_req_llm = client.req_to_llm(messages, manager.slot_status())

                res_action = response_req_llm.get("action")
                tool_input = response_req_llm.get("input", {})
                slot = response_req_llm.get("slot", "0")
                message = response_req_llm.get("message", "")

                messages.append({
                    "role": "assistant",
                    "content": json.dumps(response_req_llm, ensure_ascii=False)
                })

                if res_action == "respond":
                    print(f"\nCapri : {message}\n")
                    break
                elif res_action == "slot_check":
                    target_slot = slot

                    print(f"[*] 카프리가 {target_slot}번 슬롯의 상태를 조회합니다...")

                    slot_result = manager.get_result(target_slot)
                    result_code = slot_result[0]

                    result_msg = slot_result[1]
                    
                    print(result_msg)

                    messages.append({
                        "role": "assistant",
                        "content": f"슬롯 조회 결과 : {result_msg}"
                    })
                
                else:
                    if slot == "0":
                        print(f"[*] 카프리가 도구를 실행합니다 : {res_action} {tool_input}")
                        tool_result = base.main(res_action, tool_input, message)

                        print(f"[*] 도구 '{res_action}' 실행 결과: {tool_result}")

                        messages.append({
                            "role": "assistant",
                            "content": f"[도구 '{res_action}' 실행 결과]\n {tool_result}"
                        })
                    else:
                        print(f"[*] 카프리가 {slot}번 슬롯에서 도구를 실행합니다 : {res_action} {tool_input}")
                        
                        manager.run_slot(slot, base.main, res_action, tool_input, message)
                        
                        print(f"[*] {slot}번 슬롯에 작업이 등록되었습니다. (백그라운드 실행 중)")
                        
                        messages.append({
                            "role": "assistant",
                            "content": f"[시스템] 작업이 {slot}번 슬롯에 백그라운드 등록되었습니다."
                        })
                        break

                        
                    
    
    else:
        print(f"[-] 접속 상태에 문제가 있습니다 config.py 파일에 설정값을 확인해주세요")
        print(f"[-] 에러내용 {result_healthy_check}")