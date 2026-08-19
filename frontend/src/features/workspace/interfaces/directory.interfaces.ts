// stub temporário — remover na Task 9, quando a feature real for copiada
//
// Shape derived entirely from how `task`'s assignee.helper.ts reads these
// fields (normalizeUser/findUser) — not guessed.
export interface FractalWorkspaceDirectoryUser {
  id: string;
  name: string;
  username?: string;
  email?: string;
  image?: string;
  role?: string;
}

export interface FractalWorkspaceDirectoryAgent {
  id: string;
  name: string;
  image?: string;
  role?: string;
}
