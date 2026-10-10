export const quoted = <div id='single' class="double" data-empty="" />;

export const computed = <div id={`item-${1}`} class={["a", "b"].join(" ")} />;

export const member = <A.B.C prop="x" other={{ nested: true }}>body</A.B.C>;

declare const A: { B: { C: (p: { prop: string; other: object; children?: unknown }) => null } };
