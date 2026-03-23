''' Process
A process is an instance of a program that is being executed. Each process has its own memory space, which means that data in one process is not directly accessible to another process. This isolation provides better stability and security, as a crash in one process does not affect others.
In Python, the `multiprocessing` module allows you to create and manage separate processes. Each process runs independently and can take advantage of multiple CPU cores, enabling true parallelism. This is particularly useful for CPU-bound tasks that require significant processing power.

There are 3 ways to create a process in Python using the `multiprocessing` module- 
fork, spawn and forkserver. The default method varies by operating system-
- On Unix-like systems (Linux, macOS), the default method is `fork`.
- On Windows, the default method is `spawn`.     
'''

from multiprocessing import Process 
from multiprocessing import current_process
import os


def process_task():
    print(f"Process ID: {os.getpid()}, Parent Process ID: {os.getppid()}, Current Process Name: {current_process().name}")

process = [0] * 5

for i in range(5):
    process[i] = Process(target=process_task, name=f"Process-{i+1}")
    process[i].start()

for i in range(5):
    process[i].join()   

print(f"Main Process ID: {os.getpid()}, Current Process Name: {current_process().name}")