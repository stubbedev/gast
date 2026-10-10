export const form = (
  <input type="text" disabled readOnly required minLength={1} />
);

export const media = (
  <picture>
    <source srcSet="a.webp" type="image/webp" />
    <img src="a.png" alt="fallback" />
  </picture>
);
