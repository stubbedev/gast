declare const todos: { id: number; text: string; done: boolean }[];

export const TodoList = () => (
  <ul>
    {todos.map((todo) => (
      <li key={todo.id} class={todo.done ? "done" : "open"}>
        {todo.text}
      </li>
    ))}
  </ul>
);

export const Count = () => <p>{todos.filter((t) => t.done).length} done</p>;
