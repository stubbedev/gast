# async: functions, for loops, with statements, await
import asyncio


async def fetch(url):
    async with session.get(url) as response:
        return await response.text()


async def gather(urls):
    results = []
    async for url in urls:
        results.append(await fetch(url))
    else:
        pass
    return results


async def main():
    await asyncio.sleep(0)
    async with a, b as bs:
        pass
    async with (a, b as bs, c as cs):
        pass


value = awaitable
value = await coro
value = await coro.method(1)
