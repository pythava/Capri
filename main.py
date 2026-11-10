import random
from rich.console import Console
from rich.text import Text
from agent.orchestrator import main

console = Console()

GREEN = "#02fa6a"
PINK = "#fa023e"


BANNER_LINES = [
    r"   ______   ___      ____     ____     ____",
    r"  / ____/  /   |    / __ \   / __ \   /  _/",
    r" / /      / /| |   / /_/ /  / /_/ /   / /  ",
    r"/ /___   / ___ |  / ____/  / _, _/  _/ /   ",
    r"\____/  /_/  |_| /_/      /_/ |_|  /___/   ",
]

MODE = "diagonal"
STRIPE = 3         

def pick_color(x, y):
    if MODE == "random":
        return random.choice([GREEN, PINK])
    return GREEN if ((x + y) // STRIPE) % 2 == 0 else PINK

def print_banner():
    for y, line in enumerate(BANNER_LINES):
        text = Text()
        for x, ch in enumerate(line):
            text.append(ch, style=f"bold {pick_color(x, y)}")
        console.print(text)
    console.print("            Game Over!", style=f"dim {PINK}")


print_banner()


main()