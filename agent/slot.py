from concurrent.futures import ThreadPoolExecutor

class SlotManager:
    def __init__(self, max_slots=6):
        self.executor = ThreadPoolExecutor(max_workers=max_slots)
        self.slots = {i: None for i in range(1, max_slots + 1)}

    def run_slot(self, slot_num, target_func, *args, **kwargs):
        if slot_num not in self.slots:
            print(f"[!] {slot_num}번 슬롯은 존재하지 않습니다. (1 ~ 6 사용 가능)")
            return False

        self.slots[slot_num] = self.executor.submit(target_func, *args, **kwargs)

    def get_result(self, slot_num):
        if slot_num not in self.slots:
            print(f"[!] {slot_num}번 슬롯은 존재하지 않습니다")
            return None
        
        future = self.slots[slot_num]
        if future is None:
            print(f"[-] 슬롯 {slot_num}번은 비어 있습니다")
            return None

        if not future.done():
            print(f"[*] 슬롯 {slot_num}번 작업이 아직 진행중입니다")
            return None

        return future.result()
