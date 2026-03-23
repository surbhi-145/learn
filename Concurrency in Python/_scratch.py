from threading import *
import time

condition = Condition()
we_are_done = False

def worker_example():
    condition.acquire()
    print("Lock acquired")
    if not we_are_done:
        condition.wait()
    print("Resumed execution")

child = Thread(target=worker_example)
child.start()
time.sleep(1)

condition.acquire()
print("Lock has been acquired again")
we_are_done = True
condition.notify()
condition.release()

