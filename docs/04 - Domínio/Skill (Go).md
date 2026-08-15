---
tags: [dominio, skill, capacidade, extensibilidade]
aliases: [Skill Go, Capability Package]
fase: 8
status: especificado
origem: "[[Skill]]"
---

# Skill (Go)

> Pai: [[Workspace (Go)]] · Origem no original: [[Skill]] · Decisão: [[ADR-0015 Skills com permissões declaradas]] · Fase: 8

## Objetivo

Pacote instalável de capacidade. Uma skill não é documentação: ela traz **agentes, memórias, rotinas, coleções, views, hooks, artifacts, goals, templates, instruções e toolsets** juntos. Instalar uma skill instala uma equipe.

## Comportamento do original

Persistida em `.fractal/skills/{id}/SKILL.md`, com `references/*.md` e subdiretórios `agents/{id}/` que são o **segundo padrão** das coleções de agente, memória e rotina ([[Skill]]). `onDeleted` remove o diretório inteiro.

O `metadata` é o inventário do pacote: `rules`, `resources`, `toolsets`, `collections`, `views`, `hooks`, `artifacts`, `goals`, `templates`, `instructions`, mais `version` e `source` preenchidos automaticamente na instalação via registry.

Recursos com `always: true` são **sempre carregados**, não sob demanda — uma skill pode injetar conhecimento permanente no contexto.

## Design em Go

```go
// internal/domain/skill/entity.go

type Skill struct {
	ID string `yaml:"-" json:"id" collection:"path"`

	Name        string `yaml:"name"        json:"name"`
	Description string `yaml:"description" json:"description"`
	Active      bool   `yaml:"active"      json:"active"`

	Version string `yaml:"version,omitempty" json:"version,omitempty"` // semver, set on install
	Source  string `yaml:"source,omitempty"  json:"source,omitempty"`  // "<owner>/<repo>"
	Commit  string `yaml:"commit,omitempty"  json:"commit,omitempty"`  // added: provenance

	Permissions Permissions `yaml:"permissions" json:"permissions"` // ADR-0015
	Metadata    Metadata    `yaml:"metadata"    json:"metadata"`

	CreatedAt time.Time `yaml:"createdAt" json:"createdAt"`
	UpdatedAt time.Time `yaml:"updatedAt" json:"updatedAt"`

	Content string `yaml:"-" json:"content" collection:"content"` // the SKILL.md body
}

// Permissions is the manifest the installer verifies the package against.
// Content that exceeds the manifest is refused, not silently installed.
type Permissions struct {
	Hooks       []string      `yaml:"hooks,omitempty"       json:"hooks,omitempty"`
	Toolsets    []ToolsetPerm `yaml:"toolsets,omitempty"    json:"toolsets,omitempty"`
	Exec        []string      `yaml:"exec,omitempty"        json:"exec,omitempty"`
	Network     []string      `yaml:"network,omitempty"     json:"network,omitempty"`
	Agents      []string      `yaml:"agents,omitempty"      json:"agents,omitempty"`
	Routines    int           `yaml:"routines,omitempty"    json:"routines,omitempty"`
	Collections []string      `yaml:"collections,omitempty" json:"collections,omitempty"`
}

type Metadata struct {
	Rules        []Rule       `yaml:"rules,omitempty"        json:"rules,omitempty"`
	Resources    []Resource   `yaml:"resources,omitempty"    json:"resources,omitempty"`
	Toolsets     []ToolsetRef `yaml:"toolsets,omitempty"     json:"toolsets,omitempty"`
	Collections  []Ref        `yaml:"collections,omitempty"  json:"collections,omitempty"`
	Views        []Ref        `yaml:"views,omitempty"        json:"views,omitempty"`
	Hooks        []Ref        `yaml:"hooks,omitempty"        json:"hooks,omitempty"`
	Artifacts    []Ref        `yaml:"artifacts,omitempty"    json:"artifacts,omitempty"`
	Goals        []Ref        `yaml:"goals,omitempty"        json:"goals,omitempty"`
	Templates    []Ref        `yaml:"templates,omitempty"    json:"templates,omitempty"`
	Instructions []Ref        `yaml:"instructions,omitempty" json:"instructions,omitempty"`
}

type Resource struct {
	URI         string `yaml:"uri"         json:"uri"`
	MimeType    string `yaml:"mimeType"    json:"mimeType"`
	Description string `yaml:"description" json:"description"`
	Always      bool   `yaml:"always"      json:"always"` // permanently loaded into context
	Rules       []Rule `yaml:"rules,omitempty" json:"rules,omitempty"`
}
```

### Instalação

```go
// internal/domain/skill/install.go

type Installer struct {
	repo     Repository
	verifier Verifier
	approver approval.Approver
}

// Install fetches, verifies and applies a skill package. The order is
// deliberate: nothing touches the workspace before the manifest is verified
// and a human has consented.
func (i *Installer) Install(ctx context.Context, in InstallInput) (*Skill, error) {
	pkg, err := i.fetch(ctx, in.Source, in.Ref)
	if err != nil {
		return nil, err
	}

	// 1. Content must not exceed the declared manifest.
	diff, err := i.verifier.VerifyManifest(pkg)
	if err != nil {
		return nil, err
	}

	// 2. A human consents to the permissions. Agents cannot self-authorize.
	if !in.AcceptedAll(diff.Permissions) {
		res, err := i.approver.RequestApproval(ctx, approval.Request{
			ToolName: "skills_install", Risk: approval.High,
			Reason: diff.Render(),
		})
		if err != nil || !res.Approved {
			return nil, errInstallNotApproved(in.Source)
		}
	}

	// 3. Apply: files first, registration last, so a partial failure leaves an
	//    unregistered directory rather than a half-registered skill.
	return i.apply(ctx, pkg)
}
```

### Desinstalação

`Delete` remove o diretório inteiro — agentes, memórias e rotinas que vieram com a skill vão junto, como no original. Hooks e toolsets registrados são desregistrados **antes** da remoção dos arquivos.

## Decisões e divergências

> [!decision] Manifesto de permissões obrigatório
> Ver [[ADR-0015 Skills com permissões declaradas]]. Corrige o defeito #20.

> [!decision] Agente não instala skill de terceiro sozinho
> `skills_install` roteia por [[ADR-0007 Canal real de aprovação de tool]] com risco alto.

> [!decision] Campo `commit` para procedência
> `aos skills verify` recomputa o hash do conteúdo instalado e compara — detecta adulteração local.

> [!decision] Colisão de nome resolvida por namespace
> No original, o builtin `skills` do `incur` sobrescreve o grupo de domínio, tornando `create`/`install`/`discovery` inacessíveis pelo CLI (defeito #15). Aqui os builtins vivem sob `aos self skills`. Ver [[ADR-0016 Compatibilidade de nomes com o original]].

## Testes

- Round-trip com `metadata` completo
- Instalação com conteúdo excedendo o manifesto é recusada, nomeando o excesso
- Instalação sem consentimento é recusada
- Agente chamando `skills_install` dispara aprovação de risco alto
- Skill com agente próprio: o agente aparece em `agents list` com `skill` preenchido
- Desinstalação remove agentes, memórias e rotinas da skill e desregistra hooks
- `verify` detecta arquivo adulterado após a instalação
- `aos skills --help` mostra os comandos de domínio, não os builtins

## Critério de pronto

- [ ] Instalar uma skill que traz agente, coleção e view próprios
- [ ] Manifesto verificado contra o conteúdo
- [ ] Consentimento humano exigido
- [ ] Desinstalação limpa, com desregistro de hooks
- [ ] Sem colisão de nome com builtins
