import time
from concurrent.futures import ThreadPoolExecutor

class SlotManager:
    def __init__(self, max_slots=6):
        self.executor = ThreadPoolExecutor(max_workers=max_slots)
        self.slots = {i: {"future": None, "start_time": None} for i in range(1, max_slots + 1)}

    def run_slot(self, slot_num, target_func, *args, **kwargs):
        if slot_num not in self.slots:
            print(f"[!] {slot_num}번 슬롯은 존재하지 않습니다. (1 ~ 6 사용 가능)")
            return False

        self.slots[slot_num]["future"] = self.executor.submit(target_func, *args, **kwargs)
        self.slots[slot_num]["start_time"] = time.time()
        return True

    def get_result(self, slot_num):
        if slot_num not in self.slots:
            print(f"[!] {slot_num}번 슬롯은 존재하지 않습니다")
            return [-1, f"[!] {slot_num}번 슬롯은 존재하지 않습니다"]

        slot_data = self.slots[slot_num]
        future = slot_data["future"]

        if future is None:
            return [0, f"[-] 슬롯 {slot_num}번은 비어 있습니다"]

        elapsed_time = int(time.time() - slot_data["start_time"])

        if not future.done():
            return [1, f"[*] [진행중] {slot_num}번 슬롯 작업이 진행중입니다. [진행 시간]: {elapsed_time}"]

        result =  future.result()
        return [2, f"[V] [완료] 소요 시간: {elapsed_time}초\n결과\n{result}"]

    def slot_status(self):
        """
        0: 비어 있음 (None)
        1: 진행 중 (Running / Not done)
        2: 완료됨 (Done)
        """
        status_list = []
        for slot_num in self.slots:
            slot_data = self.slots[slot_num]
            future = slot_data["future"]

            if future is None:
                status_list.append(0)
            elif not future.done():
                status_list.append(1)
            else:
                status_list.append(2)
                
        return str(status_list) 