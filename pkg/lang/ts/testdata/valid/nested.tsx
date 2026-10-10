export const deep = (
  <a href="#x">
    <b>
      <i>
        <u>deeply nested</u>
      </i>
    </b>
  </a>
);

export const mixed = (
  <ol reversed start={3}>
    text before
    <li>one</li>
    {2}
    <li>{3}</li>
    trailing text
  </ol>
);
