''' Coroutine
A coroutine can be defined as a special function that can give up control to its caller without losing its state. 
The methods or functions that we are used to, the ones that conclusively return a value and don't remember state between invocations, can be thought of as a specialization of a coroutine, also known as subroutines.

Generators yield back a value to the invoker whereas a coroutine yields control to another coroutine and can resume execution from the point it gives up control.
A generator can't accept arguments once started whereas a coroutine can.
Generators are primarily used to simplify writing iterators. They are a type of coroutine and sometimes also called as semicoroutines.
In case, of Python, generators are used as producers of data and coroutines as consumers data.
'''

# A generator can act like a coroutine

def printer():
    item = None
    while True:
        item = yield
        print(str(item))

coroutine_object = printer()
# Moving the generator until the next yield -- priming a coroutine
next(coroutine_object)

# The send() resumes the execution of the coroutine and the snippet item = yield assigns to item whatever we pass in the send call from the main script. 
# The coroutine would wake up at the yield statement and continue execution until it encounters the next yield statement or it exits.
for i in range(0, 11):
    coroutine_object.send(i)

''' Coroutines vs Threads

One of the major benefits of coroutines over threads is that coroutines don’t use as much memory as threads do.
Coroutines don't require operating system support or invoke system calls.
Coroutines don't need to worry about synchronizing access to shared data-structures or guarding critical sections. Mutexes, semaphore and other synchronization constructs aren't required.
Coroutines are concurrent but not parallel.
'''

''' Event Loop
Event loop is a programming construct that waits for events to happen & then dispatches them to event handler. 
In Python, event loops run asynchronous tasks and callbacks, perform IO operations, run subprocesses and delegate costly function calls to pool of threads.
Device	CPU Cycles	Humanified
L1 cache	3	    1 seconds
L2 cache	14	    4.6 seconds
RAM	       250	    83 seconds
Disk	41000000	158 days
Network	240000000	2.5 years

Synchronous blocking I/O calls can be made non-blocking by either one of the following two methods:
Use threads to make blocking calls.
Convert blocking calls to nonblocking asynchronous calls.
Threads don't come cheap. Creating, maintaining and tearing down threads takes CPU cycles in addition to memory. 
'''

# To run a single event loop, you can use asyncio.run()
import asyncio
async def do_some():
    await asyncio.sleep(1)
    print("hey")

asyncio.run(do_some())

# You shouldn't need to start the event loop yourself
# Instead, use the higher level api calls to submit coroutines
# You can create an event loop, which will essentially run the coroutines
import asyncio, random
from threading import Thread
from threading import current_thread


async def do_something_important(sleep_for):
    print("Is event loop running in thread {0} = {1}\n".format(current_thread().getName(),
                                                         asyncio.get_event_loop().is_running()))
    await asyncio.sleep(sleep_for)


def launch_event_loops():
    # get a new event loop
    loop = asyncio.new_event_loop()
    # set the event loop for the current thread
    asyncio.set_event_loop(loop)
    # run a coroutine on the event loop
    loop.run_until_complete(do_something_important(random.randint(1, 5)))
    # remember to close the loop
    loop.close()

t1 = Thread(target=launch_event_loops)
t2 = Thread(target=launch_event_loops)
t1.start()
t2.start()
# This will error out, since there isn't an event loop involved 
# print("Is event loop running in thread {0} = {1}\n".format(current_thread().getName(),
#                                                         asyncio.get_event_loop().is_running()))
t1.join()
t2.join()

# There are 2 types of event loops--
#   Selector Event loop - based on selector module, default loop : poll(), select()
#   Proactor Event loop - Windows I/O completion ports & is only supported on Windows

''' yield from 
    Essentially introduced for handling nested generators
    The yield from expects an iterable on its right and runs it to exhaustion. Remember a generator is after all an iterator!
    yield from can be best thought of as creating transparent bidirectional communication between the caller and the subgenerator.
'''

def nested_generator():
    i = 0
    while i < 5:
        i += 1
        yield i

def outer_generator():
    nested_gen = nested_generator()

    for item in nested_gen:
        yield item

gen = outer_generator()
for item in gen:
    print(item)

# This can be refactored into
def outer_generator_with_yield_from():
    nested_gen = nested_generator()
    yield from nested_gen

gen_using_yield_from = outer_generator_with_yield_from()
for item in gen_using_yield_from:
    print(item)

# For my clarity-
# This also works
for item in nested_generator():
    print(item)

# With yield from + send()
def nested_generator():
    for _ in range(5):
        try:
            k = yield
            print("inner generator received = " + str(k))
        except Exception:
            print("caught an exception")
    return "Fool!"

def outer_generator():
    nested_gen = nested_generator()
    received_val = yield from nested_gen
    print("received value: " + received_val)

gen = outer_generator()
next(gen)

for i in range(5):
    try:
        if i == 1:
            gen.throw(Exception("deliberate exception"))
        else:
            gen.send(i)
    except StopIteration:
        pass

''' yielding from coroutines/futures/tasks
    A coroutine are an instance of a generator, hence we can yield from them.
    Now, PEP-492 introduced the await keyword which allows us to suspend execution until the result of await's argument (a coroutine) becomes available. 
    await is similar to yield from and borrows most of the implementation from yield from. 
'''

# legacy
# @asyncio.coroutine
# def gen_based_coro_legacy():
#     yield from asyncio.sleep(1)

async def gen_based_coro():
    await asyncio.sleep(1)
    print("slept")

gen = gen_based_coro()
loop = asyncio.new_event_loop()
loop.run_until_complete(gen)

from asyncio import Future

loop = asyncio.new_event_loop()
f = loop.create_future()
# Future has an __iter__() method
it = f.__iter__()
print(next(it))
f.set_result("hello")
print(f)
print(f.done())


# If we invoke the coroutine do_something_important() thrice serially with the values 1, 2 and 3 respectively. 
# Without using threads or multiprocessing the serial code will execute in 1 + 2 + 3 = 6 seconds, however, if we leverage async.io the same code can complete in roughly 3 seconds even though all of the invocations run in the same thread. 
# The intuition is that whenever a blocking operation is encountered the control is passed back to the event loop and execution is only resumed when the blocking operation has completed. Run the snippet below and observe the total execution time.
# If instead we use time.sleep() the execution becomes serial

import asyncio
import time
import threading

async def go_to_sleep(sleep):
    print(f"sleeping for {sleep} seconds in thread {threading.current_thread().name}")
    await asyncio.sleep(sleep)


async def do_something_important(sleep):
    await go_to_sleep(sleep)


async def main():
    now = time.time()
    await asyncio.gather(do_something_important(1), do_something_important(2), do_something_important(3))
    end = time.time()
    print(f"total time to run the script: {end - now:.2f} seconds")

asyncio.run(main())

'''Chaining coroutines
   One of the most prominent uses of coroutines is to chain them to process data pipelines. We can chain coroutines in a fashion similar to how we pipe Unix commands in a shell.
'''

async def coro3(k):
    return k + 3


async def coro2(j):
    j = j * j
    res = await coro3(j)
    return res


async def coro1():
    i = 0
    while i < 100:
        res = await coro2(i)
        print("f({0}) = {1}".format(i, res))
        i += 1


# The first 100 natural numbers evaluated for the following expression
# x^2 + 3
cr = coro1()
asyncio.run(cr)

'''Tasks
    Tasks wrap coroutines and run them in event loops. If a coroutine awaits on a Future, the Task suspends the execution of the coroutine and waits for the Future to complete. 
    When the Future is done, the execution of the wrapped coroutine resumes. Event loops use cooperative scheduling, the event loop runs one Task at a time. 
    While a Task awaits for the completion of a Future, the event loop runs other tasks, callbacks, or performs IO operations. Tasks can also be cancelled.
'''