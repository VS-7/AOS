---
tags: [adr, decisao, seguranca, rede]
aliases: [ADR-0009, Bind, 127.0.0.1]
fase: 4
status: especificado
origem: "[[Autenticação e Credenciais]]"
---

# ADR-0009 — Bind em loopback por padrão

> Pai: [[AOS]] · Origem no original: [[Autenticação e Credenciais]] · Fase: 4

## Contexto

Três decisões do original compõem um risco que nenhuma delas causaria sozinha ([[Autenticação e Credenciais]]):

1. `FRACTAL_SERVER_HOSTNAME` default `0.0.0.0` — escuta em todas as interfaces
2. `security.enabled: false` no `config.json` padrão observado
3. Playground OpenAPI com `security: (request) => true` em `/api/docs`

Resultado: **qualquer máquina da rede local alcança a API completa do workspace** — com poder de ler arquivos, executar comandos e alterar configuração. Em um café, em um coworking ou em uma rede corporativa, isso é acesso remoto não autenticado a uma ferramenta que executa shell.

Some-se o [[Tunnel (Go)]]: ativá-lo publica as mesmas rotas na internet.

E o token é aceito em query string (`?token=`), o que o grava em logs de acesso, histórico de shell e cabeçalhos `Referer`.

## Decisão

Quatro mudanças de default, todas na direção fail-closed.

**1. Bind em `127.0.0.1`.** Expor exige flag explícita e consciente:

```bash
aosd --host 0.0.0.0        # exige também --require-auth, senão aborta no boot
```

```go
// internal/transport/httpapi/server.go
func (s *Server) validateExposure() error {
	if s.host != "127.0.0.1" && s.host != "localhost" && s.host != "::1" {
		if !s.auth.Enabled {
			return apperr.New("SERVER_INSECURE_EXPOSURE").
				Issue("host", s.host).
				CTA("habilite autenticação com `aos config update --security.enabled true` " +
					"ou volte o bind para 127.0.0.1")
		}
	}
	return nil
}
```

**2. Autenticação ligada por padrão.** Na primeira execução, o onboarding gera um token de API e o grava com permissão `0600` ([[ADR-0010 Segredos com permissão restrita]]). `security.enabled: false` passa a exigir ação deliberada e um aviso no log de boot a cada start.

**3. Token só em header.** `Authorization: Bearer` e `X-Auth-Token`. **Query string não é aceita** — nem para conveniência, nem para o playground. Corrige o defeito #4.

**4. Playground autenticado e condicionado.** `/api/docs` exige o mesmo token que a API e é desabilitado quando `AOS_ENV=production`. Corrige o defeito #3.

Complemento obrigatório: o upgrade de WebSocket valida acesso ao workspace, em vez de confiar no cookie ([[Realtime WebSocket]]). Corrige o defeito #5.

## Alternativas consideradas

| Alternativa | Análise |
|---|---|
| **Manter `0.0.0.0` com aviso no log** | Avisos em log de daemon não são lidos. Default inseguro com aviso é default inseguro. |
| **Bind em `0.0.0.0` só quando o tunnel está ativo** | Ainda expõe na LAN quando o objetivo era expor na internet via tunnel — que faz proxy reverso e não precisa de bind aberto. |
| **Socket Unix em vez de TCP** | Mais seguro no caso local e elimina a porta. **Descartado**: quebra o acesso pelo navegador (a UI web em `localhost:5326`) e não existe no Windows de forma equivalente. Registrado como opção adicional, não substituta. |

## Consequências

**Positivas**
- O caso comum (uso local) fica seguro sem configuração.
- Expor a API vira decisão explícita, com autenticação obrigatória acoplada — não dá para expor e esquecer de ligar auth.
- Corrige de uma vez os defeitos #2, #3 e #4 da lista de anti-padrões.

**Negativas**
- **Quebra o caso "acessar a UI do celular na mesma rede"** sem configuração. Mitigação: `aosd --host 0.0.0.0` com mensagem de erro que ensina exatamente o que falta, e um comando `aos config expose --lan` que faz as duas coisas juntas (bind + auth + token impresso).
- **Onboarding ganha um passo.** A primeira execução precisa gerar e mostrar o token. Aceitável: é uma vez por instalação e é o que torna o resto seguro.

## Status

**Aceito.**
