async function work(items) {
  const results = [];
  for (const item of items) {
    results.push(await process(item));
  }
  for await (const chunk of stream) {
    consume(chunk);
  }
  return results;
}
const pipeline = async function* (input) {
  for await (const x of input) {
    if (x % 2) yield x ** 2;
  }
};
async () => {
  const [a, b] = await Promise.all([f(), g()]);
  return a ?? b;
};
