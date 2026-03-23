'''Creating threads

All execution starts from "main" thread ( the process launched by the python interpretter)

'''

# By creating thread object
from threading import Thread
from threading import current_thread
mythread = Thread(group=None, target=(lambda x: print(x*x)), name = "sample", args=(2,))
mythread.start()
mythread.join()

# By creating a subclass, and then an object
# CAVEAT ---> 
#   We can only override the run method
#   __init()__ must be invoked if we want to override the constructor 
#   args, kwargs don't get passed to run() method


class YOLO(Thread):

    def __init__(self, group = None, target = None, name = "YOLO", args = ..., kwargs = None, *, daemon = None):
        super().__init__(group, target, name, args, kwargs, daemon=daemon)
    
    def run(self):
        print(f"{current_thread().name} is executing")
        super().run()

mythread = YOLO(target=(lambda x: print(x*x)), args=(3,))
mythread.start()
mythread.join()
print(f"We're back in {current_thread().name}")


''' Daemon Thread

A daemon thread in Python runs in the the background. 
A Python program can exit even if a daemon thread is still running, however ending the daemon.

In contrast, if the thread was not a daemon --> Python will wait for the thread to return.
Contrary to normal background task, the daemon=True setting tells the Python interpreter: 
    "This thread is not critical. If the main program has nothing else to do, don't wait for this thread; just shut down."
'''
import time

def long_long_task():
    try:
        print("Engine is starting!!")
        start_time = time.time()
        while(time.time()-start_time < 20):
            print("Still going.. ")
    except Exception as err:
        print(f"Exception encountred, {err}")

daemonthread = Thread(target=long_long_task, daemon=True)
# daemonthread.start()
# time.sleep(3)
# print("main thread exiting!")

''' Lock

A lock object is equivalent of a mutex that you read about in OS theory. 
Any thread can acquire() on the lock object. Due to Python GIL, only 1 thread can be running at a time, 
hence only 1 thread can attempt to "acquire" the lock. 

This is in contrast to other languages, with more robust threading modules where multiple threads could be executing |
could be executing on different cores, and attempting to acquire a lock at the same time. 

Ofcourse when dealing with threads and locks, be careful of deadlocks :)
'''
from threading import Lock

lock = Lock()

def tasks():
    me = current_thread().name
    print(f"{me} attempting to acquire lock")
    lock.acquire()
    print(f"{me} in play!")
    time.sleep(1)
    lock.release()
    print(f"{me} released lock")

thread1, thread2 = Thread(target=tasks, name="thread1"), Thread(target=tasks, name="thread2")

thread1.start()
thread2.start()
thread1.join()
thread2.join()

# A re-entrant lock can be acquire > 1 times
# A rlock should be released as many times as it is acquired ( by the same thread )

from threading import RLock


def child_task():
    rlock.acquire()
    print("child task executing")
    rlock.release()


rlock = RLock()

rlock.acquire()
rlock.acquire()

rlock.release()

# COMMENT THE FOLLOWING LINE TO MAKE THE
# PROGRAM EXIT AB-NORMALLY.
rlock.release()

thread = Thread(target=child_task)
thread.start()
thread.join()

# Each reentrant lock is owned by some thread when in the locked state
# Only the owner thread is allowed to exercise a release() on the lock
# If a thread different than the owner invokes release() a RuntimeError is thrown
# If instead this was a normal lock, then it would have worked fine

rlock = RLock()

def perform_unlock():
    rlock.release()
    print("child task executing")
    rlock.release()


# reentrant lock acquired by main thread
rlock.acquire()

# let's attempt to unlock using a child thread
thread = Thread(target=perform_unlock)
thread.start()
thread.join()

''' Conditional Variable

Synchronization mechnaisms need more than just mutual exclusion.
A general need is to be able to wait for another thread to do something. 
Conditional varibales provide mutual exclusion & the ability for threads to wait for a predicate to become true. 

Locks --> can help with serial access to a shared resource, 
but they aren't enought when threads want to coordinate amongst themselves. 
e.g. thread 1 - find prime, thread 2 - print primes 
'''

# This program is written without any synchronization primitives. 
# There aren't even locks to guard writes to shared variables. 
# If there are > 2 threads, then this program will not work. 
# In Java this wouldn't have worked. 

# This is essentially a producer-consumer problem. 
# The printer_thread will constantly be polling for prime number, 
# This is called busy waiting and is highly discouraged. 
# To make the printer thread go to sleep, and only consume resources, when the condition needs acting on -->  condition. variables.



prime_number = None
exit_program = False
found_prime = False

def printer_thread():
    global prime_number
    global found_prime
    while not exit_program: 
        while not prime_number: 
            time.sleep(0.2)
        if not exit_program:
            print(prime_number)
        prime_number = None
        found_prime = False

def finder_thread():
    def is_prime(num):
        if num == 2 or num == 3:
            return True

        div = 2

        while div <= num / 2:
            if num % div == 0:
                return False
            div += 1

        return True

    global prime_number
    global found_prime

    i = 1
    while not exit_program:
        while not is_prime(i):
            i+=1
        prime_number = i
        found_prime = True
        while found_prime and not exit_program:
            time.sleep(0.1)
        i+=1

printer_worker = Thread(target=printer_thread)
finder_worker = Thread(target=finder_thread)

printer_worker.start()
finder_worker.start()

time.sleep(5)
exit_program = True

printer_worker.join()
finder_worker.join()

# Doing the same thing using condition variable 
# With condition variables, ther are 2 important methods-
#   wait() - invoked to make a thread sleep & give up resources
#   notify() - invoked by a thread when a condition becomes true
from threading import Condition
cond_var = Condition()
found_prime = False
prime_number = None
exit_program = False

def printer_thread_func():
    global prime_number
    global found_prime

    while not exit_program:
        # wait for a prime number to become
        # available for printing
        cond_var.acquire()
        if not found_prime and not exit_program:
            cond_var.wait()
        cond_var.release()
    
        if not exit_program:
            print(prime_number)
            prime_number = None
            # make sure to wake up the finder thread
            cond_var.acquire()
            found_prime = False
            cond_var.notify()
            cond_var.release()

def finder_thread_func():
    def is_prime(num):
        if num == 2 or num == 3:
            return True

        div = 2

        while div <= num / 2:
            if num % div == 0:
                return False
            div += 1

        return True
    
    global prime_number
    global found_prime

    i = 1
    while not exit_program:

        while not is_prime(i):
            i+=1

    primeHolder = i
    cond_var.acquire()
    found_prime = True
    cond_var.notify()
    cond_var.release()

    cond_var.acquire()
    while found_prime and not exit_program:
        cond_var.wait()
    cond_var.release()

    i+=1


printer_thread = Thread(target=printer_thread_func)
printer_thread.start()
finder_thread = Thread(target=finder_thread_func)
finder_thread.start()

time.sleep(3)
exit_program = True
cond_var.acquire()
cond_var.notify_all()
cond_var.release()

printer_thread.join()
finder_thread.join()



''' Details: Scenario of using notify_all()
- A thread comes along acquires the lock associated with the condition variable, and calls wait()
- The thread invoking wait() gives up the lock and goes to sleep or is taken off the CPU timeslice
- The given up lock can be reacquired by a second thread that then too calls wait(), gives up the lock, and goes to sleep.
- Notice that the lock is available for any other thread to acquire and either invoke a wait or a notify on the associated condition variable.
- Another thread comes along acquires the lock and invokes notify_all() and subsequently releases the lock.
- Note it is imperative to release the lock, otherwise the waiting threads can't reacquire the lock and return from the wait() call.
- The waiting threads are all woken up but only one of them gets to acquire the lock. This thread returns from the wait() method and proceeds forward. 
- The thread selected to acquire the lock is random and not in the order in which threads invoked wait().
- Once the thread that is the first to wake up and make progress releases the lock, other threads acquire the lock one by one and proceed ahead.
'''
from threading import Condition
from threading import Thread
from threading import current_thread
import time

flag = False

cond_var = Condition()


def child_task():
    global flag
    name = current_thread().getName()

    cond_var.acquire()
    if not flag:
        cond_var.wait()
        print("\n{0} woken up \n".format(name))

    cond_var.release()

    print("\n{0} exiting\n".format(name))


thread1 = Thread(target=child_task, name="thread1")
thread2 = Thread(target=child_task, name="thread2")
thread3 = Thread(target=child_task, name="thread3")

thread1.start()
thread2.start()
thread3.start()

cond_var.acquire()
cond_var.notify_all()
cond_var.release()

thread1.join()
thread2.join()
thread3.join()

print("main thread exits")

''' Semaphores

init(sem) = n
acquire(sem) = count-=1 ( if count > 0 return; else wait)
release(sem) = count +=1 

Semaphores are primarily used for signaling a group of threads to achieve a common goal. 
'''

from threading import Thread
from threading import Semaphore
import time


def printer_thread():
    global primeHolder

    while not exitProg:
        # wait for a prime number to become available
        sem_find.acquire()

        # print the prime number
        print(primeHolder)
        primeHolder = None

        # let the finder thread find the next prime
        sem_print.release()


def is_prime(num):
    if num == 2 or num == 3:
        return True

    div = 2

    while div <= num / 2:
        if num % div == 0:
            return False
        div += 1
    return True


def finder_thread():
    global primeHolder

    i = 1

    while not exitProg:

        while not is_prime(i):
            i += 1
            # Add a timer to slow down the thread
            # so that we can see the output
            time.sleep(.01)

        primeHolder = i

        # let the printer thread know we have
        # a prime available for printing
        sem_find.release()

        # wait for printer thread to complete
        # printing the prime number
        sem_print.acquire()

        i += 1


sem_find = Semaphore(0)
sem_print = Semaphore(0)
primeHolder = None
exitProg = False

printerThread = Thread(target=printer_thread)
printerThread.start()

finderThread = Thread(target=finder_thread)
finderThread.start()

# Let the threads run for 3 seconds
time.sleep(3)

exitProg = True

printerThread.join()
finderThread.join()


''' Events

An event object is one of the simplest primitives for synchoronization.
Internally, it has a boolean flag that can be set or unset using set() and clear().
A thread can also check if the flag is true by invoking is_set() method, and wait using wati().

An unbounded semaphore can have its internal counter incremented as many times as acquire() is invoked on it, 
whereas an event object maintains an internal boolean flag that can only flip between two state: set or unset.

Acquiring a semaphore decrements the internal counter of the semaphore whereas 
waiting on an event object doesn't change the state of the internal boolean flag.

A thread never gets blocked on wait() of an event object if the internal flag is set to true no matter how many times the thread invokes the wait() method.
'''

from threading import Thread
from threading import Event
import time


def printer_thread():
    global primeHolder

    while not exitProg:
        # wait for a prime number to become available
        prime_available.wait()

        # print the prime number
        print(primeHolder)
        primeHolder = None

        # reset the event to false
        prime_available.clear()

        # let the finder thread know that printing is done
        prime_printed.set()


def is_prime(num):
    if num == 2 or num == 3:
        return True

    div = 2

    while div <= num / 2:
        if num % div == 0:
            return False
        div += 1
    return True


def finder_thread():
    global primeHolder

    i = 1

    while not exitProg:

        while not is_prime(i):
            i += 1
            # Add a timer to slow down the thread
            # so that we can see the output
            time.sleep(.01)

        primeHolder = i

        # let the printer thread know we have
        # a prime available for printing
        prime_available.set()

        # wait for printer thread to print the prime
        prime_printed.wait()

        # reset the flag
        prime_printed.clear()

        i += 1



prime_available = Event()
prime_printed = Event()
primeHolder = None
exitProg = False

printerThread = Thread(target=printer_thread)
printerThread.start()

finderThread = Thread(target=finder_thread)
finderThread.start()

# Let the threads run for 3 seconds
time.sleep(3)

exitProg = True
prime_available.set()
prime_printed.set()

printerThread.join()
finderThread.join()

''' Barrier 

A barrier is a synchronization primitive that allows multiple threads to wait until they have all reached a certain point of execution.
Once all threads have reached the barrier, they are all released to continue their execution.
This is useful in scenarios where you want to ensure that a group of threads are synchronized at specific
points in your program.
'''

from threading import Thread
from threading import Barrier
import time

def worker(barrier, worker_id):
    print(f"Worker {worker_id} is performing some work...")
    time.sleep(1 + worker_id)  # Simulate varying workloads
    print(f"Worker {worker_id} is waiting at the barrier.")
    barrier.wait()
    print(f"Worker {worker_id} has crossed the barrier and is continuing execution.")

num_workers = 3
barrier = Barrier(num_workers)
threads = []
for i in range(num_workers):
    thread = Thread(target=worker, args=(barrier, i))
    threads.append(thread)
    thread.start()

for thread in threads:
    thread.join()

''' Timer 

A Timer is a specialized thread that executes a function after a specified interval of time.
This is useful for scheduling tasks to run after a delay without blocking the main thread.
'''

from threading import Timer 

def delayed_task(name):
    print(f"Task {name} is executed after delay.")

# Create a Timer that will execute 'delayed_task' after 5 seconds
timer = Timer(5, delayed_task, args=("MyTimerTask",))
timer.start()
