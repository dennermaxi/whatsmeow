# RProducerWA

WhatsApp HTTP gateway do RProducer — **fork do projeto open-source [Evolution Go](https://github.com/EvolutionAPI/evolution-go)** com modificações específicas pro stack do RProducer.

> ⚠️ Este é um fork independente. Não é endossado pela Evolution Foundation. O upstream Evolution Go é distribuído sob Apache License 2.0 com restrições de trademark — ver [`LICENSE`](./LICENSE) e [`TRADEMARKS.md`](./TRADEMARKS.md).

---

## Por que esse fork existe?

O RProducer roda o Evolution Go como ponte HTTP→whatsmeow pra todas as instâncias WhatsApp dos clientes. Durante uso em prod descobrimos 2 gaps no upstream que impactavam diretamente:

1. **Race condition no WebSocket producer.** A `gorilla/websocket` não é thread-safe pra writes concorrentes. Em prod 2+ goroutines chamando `WriteJSON` na mesma conn quebravam a conexão.
2. **Sem endpoint pra resolver LID → JID real.** O whatsmeow tem `LIDStore.GetPNForLID()` internamente, mas o Evolution Go não expõe via HTTP. Resultado: sync histórico do CRM caía em órfãos `@lid` sem como resolver.

Em vez de manter patches in-tree na imagem Docker (que era frágil — usávamos a tag `0.7.1-rproducer-mutex` direto do upstream), centralizamos as modificações nesse fork, com **CI próprio + Docker Hub próprio**. Permite rebase ordenado com upstream a cada release.

## Modificações em relação ao upstream

Veja [`RPRODUCER-CHANGES.md`](./RPRODUCER-CHANGES.md) pra catálogo completo.

| Mudança | Arquivos | Motivo |
|---|---|---|
| `safeConn` wrapper (mutex em writes) | `pkg/events/websocket/websocket_producer.go` | Race condition gorilla/websocket |
| `POST /chat/resolveLid` | `pkg/chat/{handler,service}/`, `pkg/routes/routes.go` | Expor LIDStore do whatsmeow |
| Branding RProducer | `README.md`, `TRADEMARKS.md`, Docker tag | Não usar marca "Evolution" |

Todo o resto é idêntico ao upstream — o objetivo é manter rebase trivial.

## Stack / Como buildar

Pré-requisitos: Docker (build multi-stage com `golang:1.25-alpine`).

```bash
# Build local pra teste
docker build --platform linux/amd64 -t local/rproducer-wa:dev .

# Build + push pra Docker Hub (faz via CI ou manual)
docker buildx build --platform linux/amd64 \
  -t dennermaxi/rproducer-wa:v0.7.1-rwa-1 \
  -t dennermaxi/rproducer-wa:latest \
  --push .
```

A imagem final tem ~80MB (alpine + binário Go estático + ffmpeg + libwebp).

## Deploy

Imagem em [`dennermaxi/rproducer-wa`](https://hub.docker.com/r/dennermaxi/rproducer-wa) no Docker Hub.

Em prod o RProducer roda como stack no EasyPanel (`evolution-evolution-go.sekraw.easypanel.host`). Trocar a tag da imagem no painel pra deployar nova versão.

## Como sincronizar com upstream

Veja [`UPSTREAM-SYNC.md`](./UPSTREAM-SYNC.md) pro processo passo-a-passo (~10min por release upstream).

## Versionamento

Tag: `vX.Y.Z-rwa-N` onde:
- `X.Y.Z` = versão do upstream Evolution Go que servimos como base
- `N` = número incremental das modificações RProducer em cima dessa base

Exemplo: `v0.7.1-rwa-1` = primeiro release nosso baseado no upstream 0.7.1.

## Licença

Apache 2.0 (herdado do upstream, ver [`LICENSE`](./LICENSE)). Marcas "Evolution", "Evolution Foundation" e "Evolution Go" permanecem propriedade da Evolution Foundation — não usamos no nome do produto/imagem.
