const items = ["a", "b"];

export const Table = () => (
  <>
    <dt>term</dt>
    <dd>definition</dd>
  </>
);

export const Wrapped = () => (
  <React.Fragment key="k">
    {items.map((i) => <span>{i}</span>)}
  </React.Fragment>
);

declare const React: { Fragment: (p: { key?: string; children?: unknown }) => null };
