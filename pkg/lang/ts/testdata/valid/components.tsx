interface Props {
  name: string;
  count: number;
}

function Greeting({ name, count }: Props) {
  return <h1 class="title">Hello {name} ({count})</h1>;
}

export default function App() {
  return <main><Greeting name="world" count={1 + 2} /></main>;
}
