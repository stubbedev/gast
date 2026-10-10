# await and async comprehensions inside async functions
async def collect(aiter, dep):
    awaited = [await item for item in aiter]
    async_drag = [item async for item in aiter]
    async_gen = (item async for item in aiter if item)
    nested = [[x async for x in row] for row in awaited]
    gen_arg = sum(x async for x in aiter())
    await dep
    return awaited, async_drag, nested


async def chained():
    async with session.get(url) as response:
        async for chunk in response:
            return chunk
