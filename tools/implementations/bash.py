import subprocess

def bash(cmd):
    result = subprocess.run(cmd, shell=True, capture_output=True, timeout=120)
    
    stdout = result.stdout.decode('utf-8', errors='ignore')
    stderr = result.stderr.decode('utf-8', errors='ignore')
    
    return stdout + stderr