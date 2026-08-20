// Todo é declarado junto de Task na fonte. Reexporta em vez de
// duplicar: uma definição, os dois caminhos que o front importa.
//
// TodoStatus não existe como export nomeado em task.interfaces.ts —
// só TodoStatusSchema (o zod schema) e Todo (o tipo inferido)
// existem. Conferido com:
//   grep -n "TodoStatus\b" src/features/task/interfaces/task.interfaces.ts
export type { Todo } from "./task.interfaces";
