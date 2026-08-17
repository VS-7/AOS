---
tags: [transporte, artifact, estatico]
aliases: [Artifacts Transport, Estáticos]
fase: 8
status: especificado
origem: "[[Artifact]]"
---

# Artifacts e Estáticos

> Pai: [[AOS]] · Origem no original: [[Camada @server]] · Ver: [[Artifact (Go)]] · Fase: 8

## Objetivo

Servir [[Artifact (Go)]]s e a SPA, com contenção de path e política de conteúdo que reconhece o que é confiável e o que não é.

## Comportamento do original

Três transportes estáticos ([[Camada @server]]): artifacts em `/v/{ws}/artifacts/{id}/*`, assets do Monaco em `/monaco/vs/*` (com rejeição de `../`), e a SPA em `/*`.

O transporte de artifacts recebe um `processSecret` gerado no boot, usado para derivar senhas — que por isso mudam a cada restart (defeito #19, corrigido em [[Artifact (Go)]]).

## Design em Go

```go
// internal/transport/artifacts/handler.go

// Handler serves artifact files. An artifact is HTML that an LLM generated,
// hosted on the same origin as the API — so the rules here are stricter than
// for the app's own assets.
func Handler(svc artifact.Service, cfg Config) http.Handler {
	r := chi.NewRouter()
	r.Get("/{workspace}/artifacts/{id}/*", func(w http.ResponseWriter, r *http.Request) {
		a, err := svc.Get(r.Context(), chi.URLParam(r, "id"))
		if err != nil {
			writeErr(w, err)
			return
		}
		if err := svc.Authorize(r.Context(), a, accessFrom(r)); err != nil {
			writeErr(w, err)
			return
		}

		p, err := pathx.ResolveInside(a.Dir, chi.URLParam(r, "*"))
		if err != nil {
			http.NotFound(w, r)
			return
		}

		setArtifactHeaders(w, a, p)
		http.ServeFile(w, r, p)
	})
	return r
}
```

### Cabeçalhos de contenção

```go
// setArtifactHeaders applies the policy that keeps generated content from
// reaching back into the workspace it was generated in.
func setArtifactHeaders(w http.ResponseWriter, a *artifact.Artifact, path string) {
	w.Header().Set("Content-Security-Policy", a.CSP()) // default: no external network, no unsafe-inline
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Cross-Origin-Resource-Policy", "same-origin")
	w.Header().Set("Referrer-Policy", "no-referrer")
	if ct := mime.TypeByExtension(filepath.Ext(path)); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", "attachment")
	}
}
```

### SPA

```go
// SPAHandler serves the embedded frontend, falling back to index.html for
// client-side routes but never for paths that look like assets — otherwise a
// missing bundle returns HTML with a 200 and the browser fails obscurely.
func SPAHandler(fsys fs.FS) http.Handler
```

O frontend é embutido com `//go:embed all:frontend/dist` no `aosd` e no `aos-desktop` — o mesmo bundle nos dois.

### Sem Monaco servido do backend

O editor Monaco, no original, é servido de `node_modules` pelo servidor. Aqui ele entra no bundle do frontend como qualquer outra dependência, o que elimina um transporte inteiro e a preocupação com traversal nele.

Implementado: `monaco-editor` + `@monaco-editor/react` no bundle Vite, self-hosted (`loader.config({ monaco })`, sem CDN), workers via imports `?worker` (`frontend/src/lib/monaco-setup.ts`). O ponto de atenção real não foi o transporte — foi a resolução de import: a partir da 0.56, o `exports` map do pacote é `"./*": "./esm/vs/*.js"`, então o especificador correto é `monaco-editor/editor/editor.worker?worker`, **sem** o prefixo `esm/vs/` que a documentação (e a maioria dos tutoriais) ainda mostra. Com o prefixo, `vite build` falha ao resolver o import (o dev server, via esbuild, ainda resolvia — só a build de produção, via Rollup, acusava o erro).

## Decisões e divergências

> [!decision] CSP estrita por padrão para artifacts
> Ver [[Artifact (Go)]]. É a diferença entre "página hospedada" e "página com acesso à sua API".

> [!decision] Cookies de sessão não vão para a rota de artifacts
> A rota é servida sem cookies de sessão. Um script num artifact não herda a sessão do usuário.

> [!decision] Monaco no bundle
> Remove um transporte e uma superfície de path traversal.

> [!decision] Contenção compartilhada
> `pathx.ResolveInside` é o mesmo helper de [[Sandbox (Go)]] e [[File (Go)]].

## Testes

- Traversal em todas as formas (`../`, `..%2f`, encoding duplo, symlink) negado
- CSP presente e restritiva por default; relaxamento só com opt-in no artifact
- Extensão desconhecida vira `attachment` com `nosniff`
- Visibilidade respeitada nas três formas
- SPA: rota de cliente cai em `index.html`; asset faltando devolve 404
- Nenhum cookie de sessão enviado na rota de artifacts

## Critério de pronto

- [ ] Artifacts servidos com contenção e CSP
- [ ] SPA embutida servida pelos dois binários
- [ ] Helper de contenção compartilhado com sandbox e file
