'''
    Yield
    The word yield is defined as to produce or provide or to give way to arguments or pressure. 
    When you are on the road you may come across "yield to pedestrians" sign boards which require you to stop and give way to pedestrians crossing the road. 
'''

def no_yield():
    return "hey"

def yes_yield():
    yield "hey"

# This returns a string
print(no_yield())

# This returns a gnerator object 
print(yes_yield())

'''
    Generator functions
    It is an iterator which helps generate values. 
'''

# calling next on a generator - returns str
# Generators are iterators, we can use loops on it
print(next(yes_yield()))

# If we call next on a generator object that hasd already produced all its values, 
# We will be thrown a StopIteration exception
iterator = yes_yield()
print(next(iterator))
try: 
    print(next(iterator))
except Exception as e: 
    print(e)

# True power of yield can be unleased with return 
# When the yield statement is encountered the state of the function is suspended and the value being yielded is returned to the caller
# At this point, the state is saved, to resume the generator function on a subsequent call to next()
# Generator functions allow us to procrastinate computing expensive values. We only compute the next value when required. 
# This makes generators memory and compute efficient. They refrain from saving long sequences in memory or doing all expensive computations upfront.
# Essentially use it for saving state. e.g. 
def show_yields():
    def _natural_nums():
        i = 0
        while True:
            i+=1 
            yield i
    yield_iterator = _natural_nums()
    print(next(yield_iterator))
    # Do something here! 
    print(next(yield_iterator))
show_yields()


# Generator Lifecycle 
# A generator goes through 4 states-
# GEN_CREATED -- When a generator onject has been returned for the first time from a generator function
# GEN_RUNNING -- When next has been invoked on the generator object and is being executed 
# GEN_SUSPENDED -- When a generator is suspended at yield
# GEN_CLOSED -- When a generator has completed execution or has been closed

from threading import Thread
import asyncio
import time
import inspect

@asyncio.coroutine
def sleeping_generator():
    time.sleep(3)
    yield None


def run_generator(gen):
    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)
    next(gen)

gen = sleeping_generator()
Thread(target=run_generator, args=(gen,)).start()
i = 0
while i != 5: 
    print("generator state " + inspect.getgeneratorstate(gen))
    time.sleep(0.1)
    i+=1

# A generator function exposes 3 functions-
# throw(), send() and close()
iterator = yes_yield()
iterator.close()
try:
    print(next(iterator))
except Exception as e:
    print(e)

# send() can be used return value to the generator
# We have to first call a next on the gen to send it in GEN_RUNNING state
# This can also be achieved via a send(None)
def generator_function():
    while True:
        item = yield
        print("received " + str(item))

gen = generator_function()
next(gen)
gen.send(37)


# In the example below, we don't receive 5
# This is because the generator suspends execution from the first yield statement
# This blocks the second yield statement
def generate_numbers():
    i = 0
    while True:
        i += 1
        yield i
        k = yield
        print(k)

generator = generate_numbers()
item = next(generator)
print(item)
generator.send(5)

# Sending & Receiving is a dance
def generate_numbers():
    i = 0
    while True:
        i += 1
        yield i
        k = yield
        print("Received in generator function: " + str(k))

generator = generate_numbers()
item = next(generator)
print("Received in main script: " + str(item))
# Nothing is received by the generator function 
# But it is again un-suspended  
item = generator.send(5)
print("Received in main script: " + str(item))
# The second send is successful
item = generator.send(5)
print("Received in main script: " + str(item))
# NOOP - Suspended again
item = next(generator)
print("Received in main script: " + str(item))
# Again there is output
item = generator.send(75)
print("Received in main script: " + str(item))
