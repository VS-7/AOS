// FractalTodo é declarado junto de FractalTask na fonte. Reexporta em vez de
// duplicar: uma definição, os dois caminhos que o front importa.
//
// FractalTodoStatus não existe como export nomeado em task.interfaces.ts —
// só FractalTodoStatusSchema (o zod schema) e FractalTodo (o tipo inferido)
// existem. Conferido com:
//   grep -n "FractalTodoStatus\b" src/features/task/interfaces/task.interfaces.ts
export type { FractalTodo } from "./task.interfaces";
