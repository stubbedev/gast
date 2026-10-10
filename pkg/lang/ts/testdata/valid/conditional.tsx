const ok = true;
const count = 2;

export const badge = ok ? <span class="ok">yes</span> : <span>no</span>;
export const line = count > 1 && count < 10 ? <em>mid</em> : null;
export const fallback = ok ? ok && <b>both</b> : <i>none</i>;
