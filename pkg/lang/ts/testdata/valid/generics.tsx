function List<T>(props: { items: T[]; render: (item: T) => string }) {
  return <ul>{props.items.map(props.render).join(", ")}</ul>;
}

const sized = <T extends object,>(value: T): T => value;
const trailing = <T,>(value: T) => value;
const constrained = <T extends string>(v: T) => v.length;

export const app = <List items={[1, 2]} render={(n) => `${n}`} />;
export { sized, trailing, constrained };

const asyncGen = async <T extends object,>(value: T): Promise<T> => Promise.resolve(value);
