export const icon = (
  <svg:path d="M0 0" stroke-width={2}>
    <svg:circle cx={1} cy={2} r={3} />
  </svg:path>
);

export const link = <svg:use xlink:href="#icon" />;
