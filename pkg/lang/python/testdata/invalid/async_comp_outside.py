# async comprehensions belong inside async functions
values = [x async for x in aiter()]
