declare const value: unknown;
declare const width: unknown;

export const el = (
  <input
    value={value as string}
    width={width as number | null}
    title={(value as string).length as unknown as string}
  />
);

export const body = <p>{value as object | null}</p>;
