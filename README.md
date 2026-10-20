# mentoria-apps

As apps da loja de lab da mentoria: o que os labs de **docker**, **pipelines** e **kubernetes**
empacotam, publicam e rodam. Cada app mora em `apps/<nome>/`, com o próprio `go.mod` e
`Dockerfile`, e o pipeline publica a imagem em `ghcr.io/karlipegomes/mentoria-apps/<nome>`,
para `linux/amd64` e `linux/arm64`.

Os labs ficam no repositório da mentoria. Aqui fica só o que eles rodam. Os defeitos de cada lab
vêm de **configuração** (variável, flag, manifesto) ou de um controle na porta de admin, e não
de imagens quebradas de propósito: assim o cenário pode ser sorteado a cada abertura, e a mesma
imagem serve a dezenas de labs.

| App | O que é | Imagem |
|---|---|---|
| [vitrine](apps/vitrine/) | o front da loja: HTTP, uploads, config, token, métricas, controles de falha | `…/vitrine` |
| [api](apps/api/) | o backend: status e produtos num Postgres | `…/api` |
| [cofre](apps/cofre/) | chave-valor em disco, uma réplica = um disco (StatefulSet) | `…/cofre` |
| [sonda](apps/sonda/) | `curl`, `dig` e `nc` para sondar de dentro da rede | `…/sonda` |

Tags publicadas: `:main` e `:sha-<curto>` a cada push na `main`, e `:1.2.3` a cada tag `v1.2.3`.
Os labs fixam o **digest**, nunca a tag.

## vitrine

Porta principal `PORTA` (8080) e porta de admin `PORTA_ADMIN` (9090). A de admin é por onde o
verify provoca o cenário: não ponha ela no Service.

| Variável | Padrão | Para quê |
|---|---|---|
| `LOJA_NOME` | — **obrigatória** | sem ela, sai com código 3 |
| `AMBIENTE` | — | `producao` torna `SEGREDO_SESSAO` e `API_URL` obrigatórias (sem uma delas, código 3) |
| `SEGREDO_SESSAO` | — | obrigatória em produção; nunca aparece em rota nenhuma |
| `LOJA_*` | — | qualquer uma aparece em `/config`, lida na partida |
| `VERSAO` | a do build | o que `/version` responde |
| `API_URL` | — | para onde `/api/` é repassado, com o caminho inteiro (`/api/status` chega como `/api/status`) |
| `DIR_UPLOADS` | `/tmp/uploads` | onde `POST /upload` grava |
| `ARQUIVO_CONFIG` | `/config/vitrine.conf` | lido **a cada pedido** de `/config` |
| `TOKEN` / `ARQUIVO_TOKEN` | — | o segredo de `/segredo`; o arquivo vence e é lido a cada pedido |
| `ATRASO_INICIO` | 0 | tempo até `/readyz` dar 200: `20`, `20s` ou faixa sorteada `20-40` |
| `DRENAGEM` | 0 | depois do SIGTERM, quanto tempo fica fora do balanceamento antes de fechar |
| `PORTA_ADMIN` | 9090 | `0` desliga |

| Rota | Resposta |
|---|---|
| `GET /` | `Loja <nome> — vitrine <versão> — pod <hostname>` |
| `GET /version`, `GET /hostname` | uma linha |
| `GET /healthz` | 200, ou 500 depois de `/admin/travar` (liveness) |
| `GET /readyz` | 503 até o fim do `ATRASO_INICIO` e durante a drenagem (readiness) |
| `GET /config` | as `LOJA_*` e o conteúdo do arquivo de config |
| `GET /segredo` | tamanho, sha256 e se termina em quebra de linha — **nunca o valor** |
| `POST /upload?nome=x`, `GET /upload/x` | grava e lê um arquivo em `DIR_UPLOADS` (até 1 MiB) |
| `GET /metrics` | `vitrine_pedidos_total{codigo}`, `vitrine_pronta`, `vitrine_info{versao}` |
| `/api/…` | repassado à `API_URL`; 503 sem ela, 502 se ela não responde |

Admin (`PORTA_ADMIN`): `POST /admin/travar` e `/admin/destravar`;
`POST /admin/memoria?mb=N&segundos=S`, um pico que o `kubectl top` quase nunca vê;
`POST /admin/erros?taxa=0.3&segundos=60`, uma fração de `GET /` responde 500.

## api

| Variável | Padrão | Para quê |
|---|---|---|
| `PORTA` | 8080 | |
| `VERSAO` | a do build | |
| `DB_URL` | — | `postgres://usuario:senha@host:5432/banco?sslmode=disable`; sem ela, sobe sem banco |
| `DB_TENTATIVAS` | 1 | tentativas de conexão na partida, a cada 2 s; esgotadas, sai com código 4 |

Rotas: `GET /api/status` (serviço, versão, pod, estado do banco), `GET /api/produtos`,
`POST /api/produtos?nome=x`, `GET /healthz`, `GET /readyz` (503 quando o banco configurado não
responde). Só existe o caminho com `/api/`: uma rota que corta o prefixo leva 404. Na primeira
partida contra um banco vazio, cria a tabela `produtos` com três itens. A senha do `DB_URL` nunca
vai para o log.

## cofre

`PORTA` (8080) e `DIR_DADOS` (`/dados`). `PUT /chave/<k>` grava o corpo, `GET /chave/<k>` lê, e
`GET /` responde `cofre <hostname>`. O 404 diz em que pod a chave não está.

## sonda

Alpine com `curl`, `dig` e `nc`, como `nobody`. Sem argumentos, dorme: dá para usar com
`kubectl exec` ou passar o comando direto, como em `kubectl run -it --rm sonda --image=… -- curl -s vitrine:8080`.

## Códigos de saída

| Código | App | Causa |
|---|---|---|
| 1 | todas | não conseguiu escutar na porta |
| 2 | todas | `PORTA` ou `PORTA_ADMIN` inválida, ou as duas iguais |
| 3 | vitrine | `LOJA_NOME` ausente, ou `SEGREDO_SESSAO`/`API_URL` com `AMBIENTE=producao` |
| 4 | api, cofre | banco inalcançável na partida; `DIR_DADOS` inutilizável |
| 5 | vitrine | `ATRASO_INICIO` ou `DRENAGEM` mal escrito |

A última linha de log sempre nomeia a mesma causa que o código.

## Rodar local

```bash
docker build --build-arg VERSAO=dev -t vitrine:dev apps/vitrine
docker run --rm -p 8080:8080 -e LOJA_NOME=demo vitrine:dev
```
