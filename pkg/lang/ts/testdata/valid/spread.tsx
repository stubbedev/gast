declare const base: { id: string; onClick: () => void };
declare const rest: Record<string, string>;

export const button = <button {...base} label="Click" data-x={1} aria-hidden />;
export const merged = <section {...rest} {...base} />;
